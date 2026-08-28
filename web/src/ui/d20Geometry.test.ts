import { describe, expect, it } from 'vitest'

import {
  ATLAS_COLUMNS,
  ATLAS_ROWS,
  D20_FACES,
  FACES,
  VERTICES,
  atlasUV,
  cross,
  dot,
  faceUp,
  sub,
} from './d20Geometry'

/**
 * The geometry is the part of the die that can be wrong without looking wrong.
 *
 * A mis-wound face is culled by the renderer or given an inward normal by the
 * collider -- a die that falls through its own floor -- and neither failure
 * announces itself as a geometry bug. None of this needs a canvas, a GPU or a
 * layout engine, which is the whole reason the shape lives in a module of its
 * own rather than inside the scene that draws it.
 */
describe('d20 geometry', () => {
  it('is twelve vertices on a unit sphere', () => {
    expect(VERTICES).toHaveLength(12)
    for (const v of VERTICES) expect(Math.hypot(...v)).toBeCloseTo(1, 12)
  })

  it('finds exactly twenty faces, each vertex in five of them', () => {
    expect(FACES).toHaveLength(D20_FACES)

    const uses = new Array(12).fill(0)
    for (const face of FACES) for (const i of face.indices) uses[i]++
    expect(uses).toEqual(new Array(12).fill(5))
  })

  it('numbers every face once, with opposite faces summing to 21', () => {
    const values = FACES.map((f) => f.value).sort((a, b) => a - b)
    expect(values).toEqual(Array.from({ length: 20 }, (_, i) => i + 1))

    for (const face of FACES) {
      const opposite = FACES.find((other) => dot(other.normal, face.normal) < -0.999)
      expect(opposite).toBeDefined()
      expect(face.value + opposite!.value).toBe(21)
    }
  })

  // The assertion that catches the failure nothing else catches. A triangle
  // wound the wrong way round still draws a triangle in the right place -- it
  // is simply facing inward, so the renderer culls it and the physics collider
  // computes a normal pointing into the solid.
  it('winds every face counter-clockwise as seen from outside', () => {
    for (const face of FACES) {
      const [a, b, c] = face.indices.map((i) => VERTICES[i]!)
      const facing = cross(sub(b!, a!), sub(c!, a!))

      expect(dot(facing, face.normal)).toBeGreaterThan(0)
    }
  })

  it('gives each face its own cell in the atlas', () => {
    const cells = FACES.map((_, i) => atlasUV(i))
    expect(ATLAS_COLUMNS * ATLAS_ROWS).toBeGreaterThanOrEqual(D20_FACES)

    for (const uv of cells) {
      expect(uv).toHaveLength(3)
      for (const [u, v] of uv) {
        expect(u).toBeGreaterThanOrEqual(0)
        expect(u).toBeLessThanOrEqual(1)
        expect(v).toBeGreaterThanOrEqual(0)
        expect(v).toBeLessThanOrEqual(1)
      }
    }

    // No two faces share a cell, or two numbers would be painted on one face.
    const centres = cells.map((uv) => uv.map(([u, v]) => `${u.toFixed(3)},${v.toFixed(3)}`).join(' '))
    expect(new Set(centres).size).toBe(D20_FACES)
  })

  describe('faceUp', () => {
    // The identity quaternion leaves the die as built, so whichever face is
    // already pointing at +y is the answer -- and it must be a real face,
    // decisively (an icosahedron has no stable edge or vertex rest).
    it('reads a face off an unrotated die', () => {
      const { value, alignment } = faceUp(0, 0, 0, 1)

      expect(value).toBeGreaterThanOrEqual(1)
      expect(value).toBeLessThanOrEqual(20)
      expect(alignment).toBeGreaterThan(0.7)
    })

    /*
     * The real test: build the rotation that puts a chosen face on top, then
     * ask which face is on top. Every one of the twenty has to answer itself.
     *
     * This is what stands between the die and its most plausible bug -- a
     * reader that is subtly out by one face, so the number the physics settled
     * on is not the number the player is told.
     */
    it.each(FACES.map((f) => f.value))('reads back face %i after rotating it upward', (value) => {
      const face = FACES.find((f) => f.value === value)!
      const up: [number, number, number] = [0, 1, 0]

      // Shortest-arc quaternion taking the face's normal to +y.
      const axis = cross(face.normal, up)
      const length = Math.hypot(...axis)
      const angle = Math.acos(Math.max(-1, Math.min(1, dot(face.normal, up))))
      const half = angle / 2
      const s = length < 1e-12 ? 0 : Math.sin(half) / length
      const q =
        length < 1e-12
          ? ([0, 0, 0, angle < 1 ? 1 : 0] as const) // already up, or exactly upside down
          : ([axis[0]! * s, axis[1]! * s, axis[2]! * s, Math.cos(half)] as const)

      const read = faceUp(q[0], q[1], q[2], q[3])

      expect(read.value).toBe(value)
      expect(read.alignment).toBeCloseTo(1, 6)
    })
  })
})
