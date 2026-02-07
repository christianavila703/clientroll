import { useState, useRef } from "react";

interface Props {
  value?: string; // current image URL (e.g. /api/files/xxx or /api/public/files/xxx)
  entityType: string; // e.g. "memberships", "bonuses", "logos"
  onChange: (url: string) => void;
  label?: string;
  hint?: string;
}

export default function ImageUpload({ value, entityType, onChange, label = "Image", hint }: Props) {
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    // Validate client-side
    if (!file.type.startsWith("image/")) {
      setError("Only images are allowed");
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      setError("File must be under 5MB");
      return;
    }

    setError("");
    setUploading(true);

    try {
      const formData = new FormData();
      formData.append("file", file);
      formData.append("entity_type", entityType);

      const token = localStorage.getItem("access_token");
      const res = await fetch("/api/tenant/upload", {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
        body: formData,
      });

      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Upload failed");

      // Use the public URL for images that need to be visible on signup pages
      onChange(data.public_url || data.url);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Upload failed");
    } finally {
      setUploading(false);
      if (inputRef.current) inputRef.current.value = "";
    }
  };

  const handleRemove = () => {
    onChange("");
  };

  return (
    <div>
      <label className="block text-[13px] font-medium text-gray-700 mb-1">{label}</label>
      {hint && <p className="text-[11px] text-gray-400 mb-2">{hint}</p>}

      {value ? (
        <div className="flex items-start gap-3">
          <img
            src={value}
            alt=""
            className="h-20 w-32 rounded-md object-cover border border-gray-200"
            onError={(e) => { (e.target as HTMLImageElement).style.display = "none"; }}
          />
          <div className="flex flex-col gap-1.5">
            <button
              type="button"
              onClick={() => inputRef.current?.click()}
              className="btn-secondary text-[12px]"
              disabled={uploading}
            >
              {uploading ? "Uploading..." : "Replace"}
            </button>
            <button
              type="button"
              onClick={handleRemove}
              className="text-[12px] text-red-600 hover:text-red-700 font-medium"
            >
              Remove
            </button>
          </div>
        </div>
      ) : (
        <div
          onClick={() => !uploading && inputRef.current?.click()}
          className="flex cursor-pointer items-center justify-center rounded-md border-2 border-dashed border-gray-200 bg-gray-50 px-4 py-6 hover:border-gray-300 hover:bg-gray-100 transition-colors"
        >
          {uploading ? (
            <div className="flex items-center gap-2 text-[13px] text-gray-500">
              <div className="h-4 w-4 animate-spin rounded-full border-2 border-gray-300 border-t-indigo-600" />
              Uploading...
            </div>
          ) : (
            <div className="text-center">
              <svg className="mx-auto h-8 w-8 text-gray-300" fill="none" stroke="currentColor" strokeWidth={1} viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 15.75l5.159-5.159a2.25 2.25 0 013.182 0l5.159 5.159m-1.5-1.5l1.409-1.41a2.25 2.25 0 013.182 0l2.909 2.91m-18 3.75h16.5a1.5 1.5 0 001.5-1.5V6a1.5 1.5 0 00-1.5-1.5H3.75A1.5 1.5 0 002.25 6v12a1.5 1.5 0 001.5 1.5zm10.5-11.25h.008v.008h-.008V8.25zm.375 0a.375.375 0 11-.75 0 .375.375 0 01.75 0z" />
              </svg>
              <p className="mt-1 text-[13px] text-gray-500">Click to upload image</p>
              <p className="text-[11px] text-gray-400">JPG, PNG, WebP, GIF · Max 5MB</p>
            </div>
          )}
        </div>
      )}

      <input
        ref={inputRef}
        type="file"
        accept="image/jpeg,image/png,image/webp,image/gif"
        onChange={handleUpload}
        className="hidden"
      />

      {error && <p className="mt-1 text-[12px] text-red-600">{error}</p>}
    </div>
  );
}
