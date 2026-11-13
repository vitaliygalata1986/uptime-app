"use client";

import { ApiError } from "./api";
import { authorizedAPIRequest } from "./auth/session";

export { fileURL } from "./api";

export interface UploadedFile {
  url: string;
  name: string;
  size: number;
  content_type: string;
}

export const MAX_FILE_BYTES = 20 * 1024 * 1024;

// Any file type is supported. The returned URL downloads it as an attachment.
export async function uploadFile(file: File): Promise<UploadedFile> {
  if (file.size > MAX_FILE_BYTES) throw new ApiError(413, "file_too_large");
  const body = new FormData();
  body.set("file", file);
  return authorizedAPIRequest<UploadedFile>("uploads", { method: "POST", data: body });
}
