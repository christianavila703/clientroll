import { useState, useRef } from "react";
import api from "../../api/client";

interface Props {
  label: string;
  hint?: string;
  entityType?: string;
  /** Direct URL of current image (used by MembershipTypeDetail) */
  currentUrl?: string | null;
  onUploaded?: (publicUrl: string) => void;
  onRemoved?: () => void;
  /** Controlled value (used by BonusesPage form) */
  value?: string;
  onChange?: (url: string) => void;
}

export default function ImageUpload({ label, hint, entityType = "general", currentUrl, onUploaded, onRemoved, value, onChange }: Props) {
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  const displayUrl = currentUrl || value || null;

  const handleFile = async (file: File) => {
    if (!file.type.startsWith("image/")) {
      setError("Only image files are allowed.");
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      setError("File must be under 5 MB.");
      return;
    }
    setError("");
    setUploading(true);
    try {
      const res = await api.uploadImage(file, entityType);
      if (onUploaded) onUploaded(res.public_url);
      if (onChange) onChange(res.public_url);
    } catch {
      setError("Upload failed. Please try again.");
    } finally {
      setUploading(false);
      if (inputRef.current) inputRef.current.value = "";
    }
  };

  const handleRemove = () => {
    if (onRemoved) onRemoved();
    if (onChange) onChange("");
  };

  return (
    <div>
      <label className="block text-[13px] font-medium text-gray-700 mb-1">{label}</label>
      {hint && <p className="text-[11px] text-gray-400 mb-2">{hint}</p>}

      {displayUrl && (
        <div className="mb-2 relative inline-block">
          <img src={displayUrl} alt="" className="h-32 rounded-lg object-cover border border-gray-200" />
          <button
            type="button"
            onClick={handleRemove}
            className="absolute -top-1.5 -right-1.5 h-5 w-5 rounded-full bg-red-500 text-white flex items-center justify-center text-[11px] leading-none hover:bg-red-600"
          >
            ×
          </button>
        </div>
      )}

      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={() => inputRef.current?.click()}
          disabled={uploading}
          className="btn-primary"
        >
          {uploading ? "Uploading..." : displayUrl ? "Replace image" : "Upload image"}
        </button>
        <input
          ref={inputRef}
          type="file"
          accept="image/jpeg,image/png,image/webp,image/gif"
          className="hidden"
          onChange={(e) => {
            const f = e.target.files?.[0];
            if (f) handleFile(f);
          }}
        />
      </div>
      {error && <p className="text-[11px] text-red-500 mt-1">{error}</p>}
    </div>
  );
}
