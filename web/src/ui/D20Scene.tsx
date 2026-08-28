import {
  Body,
  ContactMaterial,
  ConvexPolyhedron,
  Material as PhysicsMaterial,
  Plane,
  Vec3 as CannonVec3,
  World,
} from 'cannon-es'
import {
  ACESFilmicToneMapping,
  AmbientLight,
  BufferGeometry,
  CanvasTexture,
  DirectionalLight,
  Float32BufferAttribute,
  Mesh,
  MeshStandardMaterial,
  PCFSoftShadowMap,
  PerspectiveCamera,
  PlaneGeometry,
  Quaternion,
  Scene,
  ShadowMaterial,
  SRGBColorSpace,
  Vector3,
  WebGLRenderer,
} from 'three'
import { useEffect, useRef } from 'react'

import { PALETTE } from '@/theme/tokens'

import { ATLAS_COLUMNS, ATLAS_ROWS, FACES, VERTICES, atlasUV, faceUp } from './d20Geometry'

/**
 * The die itself: a real solid, in a real physics world, that you throw.
 *
 * This is the heavy half of `D20.tsx` and is **only ever loaded dynamically**
 * -- see that file for how, and why it matters. Nothing here is imported at
 * module scope by anything that ships in the main bundle.
 *
 * ## The throw decides the number
 *
 * Nothing is predetermined. The die is given the velocity your thumb gave it,
 * the simulation runs, and whatever face is pointing up when it stops is the
 * answer -- read back by `faceUp` in `d20Geometry.ts`. That is the whole point
 * of being able to throw it: a die that lands on a number chosen before it left
 * your hand is theatre, and a gentle flick that still spins wildly to reach its
 * target is theatre you can feel.
 *
 * The honest consequence is that a practised thumb has *some* influence over
 * the result, exactly as it does with a real die on a real table. That is
 * charming for a toy and would be disqualifying for a roll that mattered, so
 * if this ever becomes the app's actual roller -- a d20 against a DC, with a
 * character sheet behind it -- the outcome has to come from `domain/dice.ts`
 * and the physics demoted to an animation of a result already decided.
 * `d20()` is already wired in below for the reduced-motion path, which is the
 * shape that change would take.
 */

/** Circumradius of the die, in world units. Everything else is scaled to it. */
const DIE_RADIUS = 1

/** Vertical field of view. Fixed, because the camera height is derived from it. */
const FOV = 40

/**
 * How much of the die fits across the *narrow* side of the panel, in radii.
 *
 * The scene has no fixed size any more -- it fills whatever the carousel slide
 * gives it, which on a phone is tall and narrow. Deriving the camera height
 * from this rather than pinning it means the die is the same apparent size on
 * any panel shape, and the walls below can then be put exactly where the edges
 * of the picture are.
 */
const VISIBLE_HALF = 3

/** How high above the floor the die floats while a finger is holding it. */
const HOLD_HEIGHT = 2.4

/** Heavier than earth, because a slow die on a phone reads as a bug. */
const GRAVITY = -52

/** Below this, for this many consecutive steps, the die has stopped. */
const REST_SPEED_SQ = 0.22
const REST_STEPS = 4

/**
 * Below this the die is no longer really being thrown, it is fidgeting.
 *
 * The last phase of a throw is the part nobody watches: the die has obviously
 * finished, and spends another second nudging itself a few degrees at a time
 * before the rest test agrees. Raising gravity does not fix that -- a
 * nearly-stationary die is barely falling -- so past this threshold the
 * damping is raised hard instead, which kills the tail without making the
 * flight that precedes it feel sluggish.
 */
const SETTLING_SPEED_SQ = 7

const FLIGHT_DAMPING = { linear: 0.04, angular: 0.04 }
const SETTLING_DAMPING = { linear: 0.92, angular: 0.96 }

/** A throw that has not settled by now is nudged into settling. */
const THROW_TIMEOUT_MS = 3000

const STEP = 1 / 60

/**
 * The numerals, drawn to a canvas at load rather than shipped as an image.
 *
 * No binary asset to generate, commit and keep in step with the palette -- the
 * argument `scripts/gen-icons.mjs` exists to make, minus the file. One texture
 * and therefore one material and one draw call for the whole die.
 */
function drawAtlas(): HTMLCanvasElement {
  const CELL = 256
  const canvas = document.createElement('canvas')
  canvas.width = ATLAS_COLUMNS * CELL
  canvas.height = ATLAS_ROWS * CELL

  const ctx = canvas.getContext('2d')
  if (!ctx) return canvas

  ctx.fillStyle = PALETTE.brand
  ctx.fillRect(0, 0, canvas.width, canvas.height)

  ctx.fillStyle = PALETTE.ink
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.font = `700 ${Math.round(CELL * 0.3)}px system-ui, -apple-system, Segoe UI, sans-serif`

  FACES.forEach((face, index) => {
    const column = index % ATLAS_COLUMNS
    const row = Math.floor(index / ATLAS_COLUMNS)
    // Two thirds down the cell is the centroid of the triangle inscribed in
    // it -- see `atlasUV` -- not the middle, which would sit the numeral half
    // off the top edge of the face.
    const x = (column + 0.5) * CELL
    const y = (row + 0.64) * CELL

    ctx.fillText(String(face.value), x, y)

    // The convention every real die follows, and the reason it does: 6 and 9
    // are each other upside down, and on a solid that lands in any orientation
    // there is nothing else to tell them apart.
    if (face.value === 6 || face.value === 9) {
      const width = ctx.measureText(String(face.value)).width
      ctx.fillRect(x - width / 2, y + CELL * 0.19, width, Math.max(2, CELL * 0.022))
    }
  })

  return canvas
}

function dieGeometry(): BufferGeometry {
  const positions: number[] = []
  const uvs: number[] = []

  FACES.forEach((face, index) => {
    const uv = atlasUV(index)
    face.indices.forEach((vertex, corner) => {
      const [x, y, z] = VERTICES[vertex]!
      positions.push(x * DIE_RADIUS, y * DIE_RADIUS, z * DIE_RADIUS)
      uvs.push(uv[corner]![0], uv[corner]![1])
    })
  })

  const geometry = new BufferGeometry()
  geometry.setAttribute('position', new Float32BufferAttribute(positions, 3))
  geometry.setAttribute('uv', new Float32BufferAttribute(uvs, 2))
  // Per-face normals, which with `flatShading` is what makes the facets crisp
  // instead of a smoothly shaded ball.
  geometry.computeVertexNormals()
  return geometry
}

/** The same solid again, as something the physics engine can collide. */
function dieCollider(): ConvexPolyhedron {
  return new ConvexPolyhedron({
    vertices: VERTICES.map(([x, y, z]) => new CannonVec3(x * DIE_RADIUS, y * DIE_RADIUS, z * DIE_RADIUS)),
    faces: FACES.map((face) => [...face.indices]),
  })
}

export interface D20SceneProps {
  /** Pixel size of the box the scene fills. Not square: it fills its panel. */
  width: number
  height: number
  /** Called with the number that ended up on top. */
  onSettled: (value: number) => void
  /** Called the moment a throw leaves the hand. */
  onThrown: () => void
  /** Honour `prefers-reduced-motion`: place the die, do not throw it. */
  still: boolean
  /** Decides the face when there is no throw to decide it. */
  roll: () => number
  /**
   * The die's accessible name.
   *
   * Passed in rather than translated here so this file stays free of i18n as
   * well as of the rest of the app -- it is loaded dynamically and the less it
   * drags in with it, the smaller the chunk.
   */
  label: string
}

export default function D20Scene({ width, height, onSettled, onThrown, still, roll, label }: D20SceneProps) {
  const mount = useRef<HTMLDivElement>(null)

  // The callbacks are read from a ref inside the animation loop, so that a
  // re-render with a new closure does not tear down the whole scene -- which
  // would mean rebuilding a WebGL context on every result.
  const handlers = useRef({ onSettled, onThrown, still, roll })
  useEffect(() => {
    handlers.current = { onSettled, onThrown, still, roll }
  })

  useEffect(() => {
    const host = mount.current
    if (!host) return

    const renderer = new WebGLRenderer({ alpha: true, antialias: true })
    renderer.setPixelRatio(Math.min(2, window.devicePixelRatio))
    renderer.setSize(width, height)
    renderer.setClearAlpha(0)
    renderer.shadowMap.enabled = true
    renderer.shadowMap.type = PCFSoftShadowMap
    renderer.outputColorSpace = SRGBColorSpace
    renderer.toneMapping = ACESFilmicToneMapping
    // `touch-action: none` is load-bearing rather than tidy: without it the
    // landing carousel treats a drag across the die as a swipe between panels,
    // so throwing the die turns the page instead.
    renderer.domElement.style.touchAction = 'none'
    renderer.domElement.style.display = 'block'
    renderer.domElement.style.cursor = 'grab'
    host.appendChild(renderer.domElement)

    const scene = new Scene()
    // Straight down. The number that landed is simply the one facing the
    // camera, which is how you read a die on a table -- and is why nothing
    // here prints the result as text.
    /*
     * The camera is pulled back until the die is `VISIBLE_HALF` radii from the
     * centre of the picture to its nearest edge, and the physics box is then
     * built to match exactly what that camera can see. That is what stops the
     * die leaving the panel: the walls *are* the edges of the frame.
     *
     * Vertical field of view applies to the screen's vertical, which with this
     * camera is the world's z -- so z is derived first and x follows from the
     * aspect ratio.
     */
    const aspect = width / height
    const spread = Math.tan((FOV * Math.PI) / 360)
    const cameraHeight = aspect >= 1 ? VISIBLE_HALF / spread : VISIBLE_HALF / (spread * aspect)
    const halfDepth = cameraHeight * spread
    const halfWidth = halfDepth * aspect
    // Inset by the die's own radius, so the die bounces off the edge of the
    // picture rather than half disappearing behind it.
    const wallX = Math.max(DIE_RADIUS * 1.1, halfWidth - DIE_RADIUS)
    const wallZ = Math.max(DIE_RADIUS * 1.1, halfDepth - DIE_RADIUS)

    const camera = new PerspectiveCamera(FOV, aspect, 0.1, 200)
    camera.position.set(0, cameraHeight, 0)
    // Without this the up vector is parallel to the view direction and
    // `lookAt` has no way to orient the roll; the scene arrives empty.
    camera.up.set(0, 0, -1)
    camera.lookAt(0, 0, 0)

    scene.add(new AmbientLight(0xffffff, 1.5))
    const key = new DirectionalLight(0xffffff, 2.6)
    // Well off the camera axis on purpose: a light directly overhead would put
    // the die's shadow exactly underneath it, where a top-down camera cannot
    // see it, and the shadow is most of what gives the die a floor to sit on.
    key.position.set(halfWidth * 0.8, cameraHeight * 0.9, halfDepth * 0.6)
    key.castShadow = true
    key.shadow.mapSize.set(1024, 1024)
    key.shadow.camera.near = 0.5
    key.shadow.camera.far = cameraHeight * 3
    scene.add(key)
    const fill = new DirectionalLight(0xffffff, 0.7)
    fill.position.set(-4, 3, -3)
    scene.add(fill)

    const atlas = new CanvasTexture(drawAtlas())
    // Agreeing with the 2D canvas rather than with three's default, because
    // the numerals are drawn with `fillText` and `atlasUV` measures v downward
    // to match. See d20Geometry.ts.
    atlas.flipY = false
    atlas.colorSpace = SRGBColorSpace
    atlas.anisotropy = renderer.capabilities.getMaxAnisotropy()

    const geometry = dieGeometry()
    const material = new MeshStandardMaterial({
      map: atlas,
      flatShading: true,
      roughness: 0.42,
      metalness: 0.08,
    })
    const die = new Mesh(geometry, material)
    die.castShadow = true
    scene.add(die)

    // A plane that catches the shadow and is otherwise not there, so the die
    // sits on the page's own background in either colour scheme rather than on
    // a surface this component had to pick a colour for.
    const floor = new Mesh(new PlaneGeometry(200, 200), new ShadowMaterial({ opacity: 0.26 }))
    floor.rotation.x = -Math.PI / 2
    floor.receiveShadow = true
    scene.add(floor)
    scene.background = null

    const world = new World({ gravity: new CannonVec3(0, GRAVITY, 0) })
    world.allowSleep = true

    const solid = new PhysicsMaterial('solid')
    world.addContactMaterial(
      // Bouncy on purpose: the die is meant to carom off the edges of the
      // panel. The long tail this would normally cause is dealt with by the
      // damping switch below rather than by damping the throw itself.
      new ContactMaterial(solid, solid, { friction: 0.22, restitution: 0.62 }),
    )

    // Floor, four walls and a lid. The walls are why the die stays on screen
    // instead of being flung out of frame by an enthusiastic thumb; the lid is
    // why it cannot be thrown over them.
    const lid = cameraHeight - DIE_RADIUS * 1.5
    const bounds: [CannonVec3, CannonVec3][] = [
      [new CannonVec3(0, 0, 0), new CannonVec3(-Math.PI / 2, 0, 0)],
      [new CannonVec3(0, lid, 0), new CannonVec3(Math.PI / 2, 0, 0)],
      [new CannonVec3(-wallX, 0, 0), new CannonVec3(0, Math.PI / 2, 0)],
      [new CannonVec3(wallX, 0, 0), new CannonVec3(0, -Math.PI / 2, 0)],
      [new CannonVec3(0, 0, -wallZ), new CannonVec3(0, 0, 0)],
      [new CannonVec3(0, 0, wallZ), new CannonVec3(0, Math.PI, 0)],
    ]
    for (const [position, rotation] of bounds) {
      const wall = new Body({ mass: 0, shape: new Plane(), material: solid })
      wall.position.copy(position)
      wall.quaternion.setFromEuler(rotation.x, rotation.y, rotation.z)
      world.addBody(wall)
    }

    const body = new Body({ mass: 1, shape: dieCollider(), material: solid })
    body.linearDamping = FLIGHT_DAMPING.linear
    body.angularDamping = FLIGHT_DAMPING.angular
    body.sleepSpeedLimit = 0.2
    body.position.set(0, DIE_RADIUS * 1.2, 0)
    world.addBody(body)

    /** Rest the die on the floor with `value` facing up, and report it. */
    const place = (value: number) => {
      const face = FACES.find((each) => each.value === value) ?? FACES[0]!
      const quaternion = new Quaternion().setFromUnitVectors(
        new Vector3(...face.normal),
        new Vector3(0, 1, 0),
      )
      body.velocity.setZero()
      body.angularVelocity.setZero()
      body.quaternion.set(quaternion.x, quaternion.y, quaternion.z, quaternion.w)
      // The face opposite the one on top is the one resting on the floor, so
      // the centre sits exactly an inradius above it.
      body.position.set(0, DIE_RADIUS * 0.7947, 0)
      body.wakeUp()
    }

    let thrown = false
    let restSteps = 0
    let thrownAt = 0

    const settle = () => {
      thrown = false
      restSteps = 0
      handlers.current.onSettled(faceUp(body.quaternion.x, body.quaternion.y, body.quaternion.z, body.quaternion.w).value)
    }

    const launch = (vx: number, vy: number, vz: number, spin: number, fromHand = false) => {
      if (handlers.current.still) {
        // Reduced motion: no flight, no tumble. The die is simply placed on a
        // number, which `domain/dice.ts` chooses since no throw did.
        const value = handlers.current.roll()
        place(value)
        handlers.current.onThrown()
        handlers.current.onSettled(value)
        return
      }

      body.type = Body.DYNAMIC
      body.updateMassProperties()
      body.allowSleep = true
      body.wakeUp()
      // A die let go of carries on from where your hand was. Only a throw with
      // no hand behind it -- a tap, a keypress -- gets to start somewhere new.
      if (!fromHand) {
        body.position.set((Math.random() - 0.5) * wallX, Math.min(lid - 0.2, 3.4), wallZ * 0.55)
        body.quaternion.setFromEuler(Math.random() * 6.28, Math.random() * 6.28, Math.random() * 6.28)
      }
      body.velocity.set(vx, vy, vz)
      body.angularVelocity.set(
        (Math.random() - 0.5) * spin,
        (Math.random() - 0.5) * spin,
        (Math.random() - 0.5) * spin,
      )
      thrown = true
      restSteps = 0
      thrownAt = performance.now()
      handlers.current.onThrown()
    }

    /**
     * Where a finger is currently holding the die, or null if nobody is.
     *
     * A forward declaration, because the pointer handlers that answer it are
     * set up after the loop that asks -- the loop has to exist before there is
     * anything to drag.
     */
    let hold: () => { x: number; z: number } | null = () => null

    let frame = 0
    let last = performance.now()

    const tick = () => {
      frame = requestAnimationFrame(tick)
      const now = performance.now()
      world.step(STEP, Math.min(0.05, (now - last) / 1000), 3)
      last = now

      // A held die is placed rather than simulated: the finger decides where
      // it is, and the physics only takes over again when it is let go.
      const grip = hold()
      if (grip) {
        body.position.set(grip.x, HOLD_HEIGHT, grip.z)
        body.velocity.setZero()
      }

      die.position.set(body.position.x, body.position.y, body.position.z)
      die.quaternion.set(body.quaternion.x, body.quaternion.y, body.quaternion.z, body.quaternion.w)

      if (thrown && !grip) {
        // Swap to the heavy damping as soon as the throw stops being a throw.
        // See SETTLING_SPEED_SQ -- this is what stops the die dawdling.
        const busy =
          body.velocity.lengthSquared() + body.angularVelocity.lengthSquared() >= SETTLING_SPEED_SQ
        const damping = busy ? FLIGHT_DAMPING : SETTLING_DAMPING
        body.linearDamping = damping.linear
        body.angularDamping = damping.angular

        const still =
          body.velocity.lengthSquared() < REST_SPEED_SQ &&
          body.angularVelocity.lengthSquared() < REST_SPEED_SQ
        restSteps = still ? restSteps + 1 : 0

        if (restSteps >= REST_STEPS) settle()
        else if (now - thrownAt > THROW_TIMEOUT_MS) {
          // Wedged against a wall, or bouncing forever on a bad contact. Drop
          // it flat rather than leaving the player with a die that never
          // answers -- reading a leaning die would give a number no face is
          // actually showing.
          place(handlers.current.roll())
          settle()
        }
      }

      renderer.render(scene, camera)
    }

    place(20)
    frame = requestAnimationFrame(tick)

    // --- the throw -------------------------------------------------------

    /**
     * A throw with no direction behind it: a tap, or a keypress.
     *
     * The die must never be a control that did nothing, so a press that
     * carries no fling still throws -- from a random direction, so that
     * tapping repeatedly is not the same throw over and over.
     */
    const toss = () => {
      const angle = Math.random() * Math.PI * 2
      launch(Math.cos(angle) * 11, 5.5, Math.sin(angle) * 11 - 4, 105)
    }

    const canvas = renderer.domElement

    /**
     * Where a screen point lands on the plane the die is held at.
     *
     * The camera looks straight down, so this is a similar-triangles problem
     * rather than a raycast: the picture is `spread` wide per unit of distance,
     * and the held plane is `cameraHeight - HOLD_HEIGHT` away. Screen up is
     * world -z, which is what `camera.up` was set to, hence the negation.
     */
    const toWorld = (clientX: number, clientY: number) => {
      const rect = canvas.getBoundingClientRect()
      const nx = ((clientX - rect.left) / rect.width) * 2 - 1
      const ny = 1 - ((clientY - rect.top) / rect.height) * 2
      const depth = (cameraHeight - HOLD_HEIGHT) * spread
      return { x: nx * depth * aspect, z: -ny * depth }
    }

    let held = false
    /** Where the die was, and where the finger was, when the grab started. */
    let grabOffset = { x: 0, z: 0 }
    let target = { x: 0, z: 0 }
    let trail: { x: number; z: number; t: number }[] = []

    const down = (event: PointerEvent) => {
      // Stops the carousel underneath from reading the same gesture as a swipe
      // between panels.
      event.stopPropagation()
      canvas.setPointerCapture(event.pointerId)
      canvas.style.cursor = 'grabbing'

      const at = toWorld(event.clientX, event.clientY)
      /*
       * The die moves by the *delta* of the drag rather than to the finger.
       *
       * Snapping it under the finger would be simpler and is what a first
       * draft did, but pressing anywhere in a tall panel then teleports the
       * die across the screen before you have moved at all. Tracking the
       * offset means a press picks the die up wherever it is and a drag
       * carries it exactly as far as the finger went.
       */
      grabOffset = { x: body.position.x - at.x, z: body.position.z - at.z }
      target = { x: body.position.x, z: body.position.z }

      held = true
      thrown = false
      // Kinematic while held: gravity stops applying, the walls stop pushing
      // back, and the position this sets each frame is simply obeyed.
      body.type = Body.KINEMATIC
      body.updateMassProperties()
      body.allowSleep = false
      body.wakeUp()
      body.velocity.setZero()
      // Kept turning in the hand. A die held perfectly still reads as frozen.
      body.angularVelocity.set(1.6, 2.4, 1.1)

      trail = [{ x: target.x, z: target.z, t: performance.now() }]
    }

    const move = (event: PointerEvent) => {
      if (!held) return
      event.stopPropagation()

      const at = toWorld(event.clientX, event.clientY)
      target = {
        x: Math.max(-wallX, Math.min(wallX, at.x + grabOffset.x)),
        z: Math.max(-wallZ, Math.min(wallZ, at.z + grabOffset.z)),
      }

      trail.push({ x: target.x, z: target.z, t: performance.now() })
      // Only the tail of the gesture decides the throw: what the hand was
      // doing at the moment of release is the fling, and averaging in the slow
      // beginning of a long drag would flatten every throw towards nothing.
      if (trail.length > 5) trail.shift()
    }

    const up = (event: PointerEvent) => {
      if (!held) return
      held = false
      canvas.releasePointerCapture(event.pointerId)
      canvas.style.cursor = 'grab'

      const first = trail[0]
      const last = trail[trail.length - 1]
      if (!first || !last) {
        toss()
        return
      }

      // World units per second, straight out of the drag -- no pixel scaling,
      // because `toWorld` already did that conversion.
      const seconds = Math.max(0.016, (last.t - first.t) / 1000)
      const vx = ((last.x - first.x) / seconds) * 1.5
      const vz = ((last.z - first.z) / seconds) * 1.5
      const speed = Math.hypot(vx, vz)

      if (speed < 1.2) {
        // Picked up and put down rather than thrown. Throw it anyway.
        toss()
        return
      }

      const capped = Math.min(30, speed)
      const scale = capped / speed
      launch(vx * scale, 3.2 + capped * 0.16, vz * scale, Math.min(130, 55 + capped * 3), true)
    }

    const press = (event: KeyboardEvent) => {
      if (event.key !== 'Enter' && event.key !== ' ') return
      event.preventDefault()
      toss()
    }

    host.addEventListener('keydown', press)
    canvas.addEventListener('pointerdown', down)
    canvas.addEventListener('pointermove', move)
    canvas.addEventListener('pointerup', up)
    canvas.addEventListener('pointercancel', up)

    /** Read by the animation loop so it can drive a held die. */
    hold = () => (held ? target : null)

    return () => {
      cancelAnimationFrame(frame)
      host.removeEventListener('keydown', press)
      canvas.removeEventListener('pointerdown', down)
      canvas.removeEventListener('pointermove', move)
      canvas.removeEventListener('pointerup', up)
      canvas.removeEventListener('pointercancel', up)
      host.removeChild(canvas)
      // WebGL holds GPU memory the garbage collector cannot see, so every one
      // of these is a leak if it is skipped -- and this component mounts and
      // unmounts every time the sheet is opened.
      geometry.dispose()
      material.dispose()
      atlas.dispose()
      floor.geometry.dispose()
      floor.material.dispose()
      renderer.dispose()
    }
  }, [width, height])

  return (
    // Focusable and named, because everything above is pointer input: a die
    // that can only be thrown with a thumb is a die some people cannot throw
    // at all. `button` rather than `img` -- it does something when pressed.
    <div
      ref={mount}
      role="button"
      tabIndex={0}
      aria-label={label}
      style={{ width, height, outlineOffset: -2 }}
    />
  )
}
