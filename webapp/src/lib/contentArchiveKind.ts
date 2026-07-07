export type ContentArchiveKind = "auto" | "car" | "track";
export type DetectedZipContentKind = ContentArchiveKind | "mixed" | "unknown";

const EOCD_SIGNATURE = 0x06054b50;
const CENTRAL_FILE_SIGNATURE = 0x02014b50;
const EOCD_MIN_BYTES = 22;
const ZIP_MAX_COMMENT_BYTES = 0xffff;
const MAX_CENTRAL_DIRECTORY_BYTES = 32 * 1024 * 1024;

export async function resolveUploadArchiveKind(file?: File): Promise<ContentArchiveKind> {
  if (!file || !file.name.toLowerCase().endsWith(".zip")) return "auto";

  let detected: DetectedZipContentKind;
  try {
    detected = detectArchiveKindFromPaths(await listZipFileNames(file));
  } catch {
    return "auto";
  }
  if (detected === "mixed") {
    throw new Error("Archive contains both car and track content. Upload mixed content as separate archives so each file can be installed to the right folder.");
  }
  if (detected === "unknown") {
    throw new Error("No valid car or track content was found in the archive. Expected an Assetto Corsa car with ui/ui_car.json or track with ui/ui_track.json.");
  }
  return detected;
}

export function detectArchiveKindFromPaths(paths: string[]): DetectedZipContentKind {
  let car = false;
  let track = false;

  for (const rawPath of paths) {
    const path = rawPath.replaceAll("\\", "/").toLowerCase();
    if (/(^|\/)ui\/(?:dlc_)?ui_car\.json$/.test(path)) car = true;
    if (/(^|\/)ui\/(?:dlc_)?ui_track\.json$/.test(path)) track = true;
  }

  if (car && track) return "mixed";
  if (car) return "car";
  if (track) return "track";
  return "unknown";
}

async function listZipFileNames(file: File): Promise<string[]> {
  const tailSize = Math.min(file.size, EOCD_MIN_BYTES + ZIP_MAX_COMMENT_BYTES);
  const tailOffset = file.size - tailSize;
  const tail = new Uint8Array(await file.slice(tailOffset).arrayBuffer());
  const tailView = new DataView(tail.buffer, tail.byteOffset, tail.byteLength);
  const eocdOffset = findEndOfCentralDirectory(tailView);
  if (eocdOffset < 0) throw new Error("The selected zip archive could not be inspected.");

  const centralDirectorySize = tailView.getUint32(eocdOffset + 12, true);
  const centralDirectoryOffset = tailView.getUint32(eocdOffset + 16, true);
  if (
    centralDirectorySize === 0xffffffff ||
    centralDirectoryOffset === 0xffffffff ||
    centralDirectorySize > MAX_CENTRAL_DIRECTORY_BYTES ||
    centralDirectoryOffset + centralDirectorySize > file.size
  ) {
    throw new Error("The selected zip archive could not be inspected.");
  }

  const centralDirectory = new Uint8Array(
    await file.slice(centralDirectoryOffset, centralDirectoryOffset + centralDirectorySize).arrayBuffer(),
  );
  return readCentralDirectoryNames(centralDirectory);
}

function findEndOfCentralDirectory(view: DataView): number {
  for (let offset = view.byteLength - EOCD_MIN_BYTES; offset >= 0; offset--) {
    if (view.getUint32(offset, true) === EOCD_SIGNATURE) return offset;
  }
  return -1;
}

function readCentralDirectoryNames(bytes: Uint8Array): string[] {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const decoder = new TextDecoder();
  const names: string[] = [];
  let offset = 0;

  while (offset + 46 <= view.byteLength && view.getUint32(offset, true) === CENTRAL_FILE_SIGNATURE) {
    const fileNameLength = view.getUint16(offset + 28, true);
    const extraLength = view.getUint16(offset + 30, true);
    const commentLength = view.getUint16(offset + 32, true);
    const fileNameStart = offset + 46;
    const fileNameEnd = fileNameStart + fileNameLength;
    if (fileNameEnd > bytes.length) break;
    names.push(decoder.decode(bytes.subarray(fileNameStart, fileNameEnd)));
    offset = fileNameEnd + extraLength + commentLength;
  }

  return names;
}
