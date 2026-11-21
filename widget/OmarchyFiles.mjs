import { maxRecordBytes, recordId } from "./Records.mjs"

export function listing(output) {
  const fields = output.split("\0")
  const files = Object.create(null)
  for (let i = 0; i + 1 < fields.length && i < 256; i += 2) {
    const id = recordId(fields[i])
    if (id !== null) files[id] = Number(fields[i + 1]) <= maxRecordBytes
  }
  return files
}
