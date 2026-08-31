/**
 * The shape of a twenty-sided die, derived once and used by three things.
 *
 * It is a module of its own rather than part of `D20Scene.tsx` because three
 * separate consumers need the same solid and must agree about it exactly: the
 * rendered mesh, the physics collider that stands in for it, and the reader
 * that decides which face ended up on top. A collider that disagreed with its
 * mesh by one flipped winding is a die that visibly bounces off nothing.
 *
 * Framework-free -- no three, no cannon, no React. That is what lets
 * `d20Geometry.test.ts` assert the geometry directly instead of through a
 * canvas jsdom cannot rasterise.
 */

export const D20_FACES = 20

const PHI = (1 + Math.sqrt(5)) / 2

/** Squared edge length of the vertex set below; a face is three of these. */
const EDGE_SQ = 4

export type Vec3 = readonly [number, number, number]

export const sub = (a: Vec3, b: Vec3): Vec3 => [a[0] - b[0], a[1] - b[1], a[2] - b[2]]
export const dot = (a: Vec3, b: Vec3): number => a[0] * b[0] + a[1] * b[1] + a[2] * b[2]
export const cross = (a: Vec3, b: Vec3): Vec3 => [
  a[1] * b[2] - a[2] * b[1],
  a[2] * b[0] - a[0] * b[2],
  a[0] * b[1] - a[1] * b[0],
]
const scaled = (a: Vec3, k: number): Vec3 => [a[0] * k, a[1] * k, a[2] * k]
const unit = (a: Vec3): Vec3 => scaled(a, 1 / Math.hypot(a[0], a[1], a[2]))

/**
 * The twelve vertices -- every cyclic permutation of `(0, ±1, ±φ)` -- scaled so
 * the solid's circumradius is exactly 1.
 *
 * Normalising here rather than at each call site means the die is a unit
 * object everywhere: the physics world, the camera framing and the mesh all
 * measure it the same way, and "how big is the die" is a single multiplier at
 * the top of the scene instead of a factor threaded through three files.
 */
export const VERTICES: readonly Vec3[] = (() => {
  const raw: Vec3[] = []
  for (const p of [1, -1]) {
    for (const q of [1, -1]) {
      raw.push([0, p, q * PHI], [p, q * PHI, 0], [q * PHI, 0, p])
    }
  }
  const circumradius = Math.hypot(...raw[0]!)
  return raw.map((v) => scaled(v, 1 / circumradius))
})()

export interface D20Face {
  /** The number printed on it. Opposite faces sum to 21, as on a real die. */
  value: number
  /** Indices into `VERTICES`, wound counter-clockwise seen from outside. */
  indices: readonly [number, number, number]
  /** Unit outward normal. */
  normal: Vec3
}

export const FACES: readonly D20Face[] = (() => {
  const distSq = (a: Vec3, b: Vec3) => dot(sub(a, b), sub(a, b))
  // Edge length shrank with the vertices above, so the test shrinks with it.
  const edgeSq = EDGE_SQ / Math.hypot(1, PHI) ** 2

  const found: { indices: [number, number, number]; normal: Vec3 }[] = []
  for (let i = 0; i < VERTICES.length; i++) {
    for (let j = i + 1; j < VERTICES.length; j++) {
      if (Math.abs(distSq(VERTICES[i]!, VERTICES[j]!) - edgeSq) > 1e-9) continue
      for (let k = j + 1; k < VERTICES.length; k++) {
        if (Math.abs(distSq(VERTICES[j]!, VERTICES[k]!) - edgeSq) > 1e-9) continue
        if (Math.abs(distSq(VERTICES[i]!, VERTICES[k]!) - edgeSq) > 1e-9) continue

        const a = VERTICES[i]!
        const b = VERTICES[j]!
        const c = VERTICES[k]!
        // The solid is centred on the origin, so a face's outward normal is
        // just the direction of its own centroid. No winding needed to find
        // it -- which is what lets the winding be *fixed* against it below.
        const normal = unit([(a[0] + b[0] + c[0]) / 3, (a[1] + b[1] + c[1]) / 3, (a[2] + b[2] + c[2]) / 3])
        // Counter-clockwise seen from outside, which both consumers require
        // and neither checks: three would cull the face as a backface, and
        // cannon's ConvexPolyhedron would compute an inward normal and let the
        // die fall through the floor it is standing on.
        const wound: [number, number, number] =
          dot(cross(sub(b, a), sub(c, a)), normal) > 0 ? [i, j, k] : [i, k, j]

        found.push({ indices: wound, normal })
      }
    }
  }

  // Opposite faces sum to 21. Ten passes, each claiming a face and its
  // antipode -- the difference between a d20 and twenty numbers glued to a
  // solid, and visible whenever more than one face is.
  const values = new Array<number>(found.length).fill(0)
  let next = 1
  for (let i = 0; i < found.length; i++) {
    if (values[i]) continue
    const opposite = found.findIndex((f, k) => k !== i && dot(f.normal, found[i]!.normal) < -0.999)
    values[i] = next
    if (opposite >= 0) values[opposite] = D20_FACES + 1 - next
    next++
  }

  return found.map((face, i) => ({ ...face, value: values[i]! }))
})()

if (FACES.length !== D20_FACES) {
  throw new Error(`d20 geometry produced ${FACES.length} faces, not ${D20_FACES}`)
}

/**
 * Where each face's numeral sits in the texture atlas: a 5x4 grid of cells.
 *
 * The atlas is drawn to a canvas at runtime rather than shipped as an image,
 * so there is no binary asset to generate, commit and keep in step with the
 * palette -- the same argument `scripts/gen-icons.mjs` exists to make, minus
 * the file.
 */
export const ATLAS_COLUMNS = 5
export const ATLAS_ROWS = 4

/**
 * UVs for one face, as three `(u, v)` pairs matching its wound vertices.
 *
 * The triangle is inscribed in its cell with a margin, apex at the top. `v`
 * runs downward to match a canvas, which is why the scene sets
 * `texture.flipY = false`: agreeing with the 2D drawing API is worth more than
 * agreeing with three's default, because the numerals are drawn with `fillText`
 * and read wrong if the two disagree.
 *
 * **The base runs left before right, and that is the whole of whether a numeral
 * reads or is mirrored.** A face is wound counter-clockwise seen from outside,
 * so the three corners of the cell have to be listed counter-clockwise *as a
 * human sees the image* -- and because `v` points down, that is apex, then
 * bottom-left, then bottom-right. Listing the base the other way round is an
 * orientation-reversing map: every triangle lands in the right place, at the
 * right size, with its number printed backwards. `d20Geometry.test.ts` pins the
 * sign, because a mirrored 7 is the kind of wrong that only a person looking at
 * the die can see.
 */
export function atlasUV(faceIndex: number): [number, number][] {
  const column = faceIndex % ATLAS_COLUMNS
  const row = Math.floor(faceIndex / ATLAS_COLUMNS)
  const w = 1 / ATLAS_COLUMNS
  const h = 1 / ATLAS_ROWS
  // Apex up, base down, inset so the numeral never touches a cell edge.
  const local: [number, number][] = [
    [0.5, 0.06],
    [0.05, 0.94],
    [0.95, 0.94],
  ]
  return local.map(([u, v]) => [column * w + u * w, row * h + v * h])
}

/**
 * Which number is face up, given the die's orientation as a quaternion.
 *
 * Takes the quaternion's components rather than a `THREE.Quaternion` so that
 * this file stays free of the renderer, and so a test can ask the question
 * with four numbers.
 *
 * Returns the face whose rotated normal points most nearly at `+y`. An
 * icosahedron has no stable edge or vertex rest, so on a settled die the best
 * match is always decisive -- `alignment` is returned alongside so the caller
 * can tell a clean landing from a die still leaning on a wall.
 */
export function faceUp(
  x: number,
  y: number,
  z: number,
  w: number,
): { value: number; alignment: number } {
  let best = FACES[0]!
  let bestUp = -Infinity

  for (const face of FACES) {
    // v' = v + 2w(q x v) + 2(q x (q x v)), the standard form. Written out
    // rather than pulled from three, because this module stays free of the
    // renderer -- and preferred over expanding q*v*q^-1 by hand, which is the
    // same rotation with four more chances to transpose a sign.
    const q: Vec3 = [x, y, z]
    const t = scaled(cross(q, face.normal), 2)
    const rotated: Vec3 = [
      face.normal[0] + w * t[0] + cross(q, t)[0],
      face.normal[1] + w * t[1] + cross(q, t)[1],
      face.normal[2] + w * t[2] + cross(q, t)[2],
    ]

    if (rotated[1] > bestUp) {
      bestUp = rotated[1]
      best = face
    }
  }

  return { value: best.value, alignment: bestUp }
}
