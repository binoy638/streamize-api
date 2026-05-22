import { type FormEvent, useEffect, useMemo, useState } from "react";

import * as api from "../lib/api";
import { Button, Field, Input, Modal, Select } from "./ui";

type ShareTarget = {
  catalogId: string;
  title: string;
  torrentId: string;
  primaryFileId: string;
  fileCount: number;
};

const EXPIRY_OPTIONS = [
  { label: "24 hours", hours: 24 },
  { label: "7 days", hours: 24 * 7 },
  { label: "30 days", hours: 24 * 30 },
];

type CreateShareModalProps = {
  open: boolean;
  onClose: () => void;
  // When set, the modal shares this catalog item and hides the picker.
  presetItemId?: string;
  onCreated?: (share: api.ShareSummary) => void;
};

export function CreateShareModal({ open, onClose, presetItemId, onCreated }: CreateShareModalProps) {
  const [targets, setTargets] = useState<ShareTarget[]>([]);
  const [loading, setLoading] = useState(false);
  const [catalogId, setCatalogId] = useState("");
  const [scope, setScope] = useState<api.ShareScope>("file");
  const [expiryHours, setExpiryHours] = useState(EXPIRY_OPTIONS[0].hours);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [created, setCreated] = useState<api.ShareSummary | null>(null);

  const selected = useMemo(() => targets.find((target) => target.catalogId === catalogId), [targets, catalogId]);

  // Load shareable library items each time the modal opens, and reset state.
  useEffect(() => {
    if (!open) {
      return;
    }
    setError("");
    setCreated(null);
    setBusy(false);
    setExpiryHours(EXPIRY_OPTIONS[0].hours);
    setLoading(true);

    let canceled = false;
    api
      .listLibrary()
      .then((items) => {
        if (canceled) {
          return;
        }
        const shareable = items
          .map((item): ShareTarget | null => {
            const primary = item.files.find((file) => Boolean(file.hlsPath) || file.directPlayable);
            if (!primary) {
              return null;
            }
            return {
              catalogId: item.id,
              title: item.title,
              torrentId: primary.torrentId,
              primaryFileId: primary.id,
              fileCount: item.files.length,
            };
          })
          .filter((target): target is ShareTarget => target !== null);

        setTargets(shareable);
        const initial = shareable.find((target) => target.catalogId === presetItemId) || shareable[0];
        setCatalogId(initial?.catalogId || "");
        setScope(initial && initial.fileCount > 1 ? "torrent" : "file");
        if (presetItemId && !shareable.some((target) => target.catalogId === presetItemId)) {
          setError("This title has no files ready to share yet.");
        }
      })
      .catch((err) => {
        if (!canceled) {
          setError(err instanceof Error ? err.message : "Unable to load your library.");
        }
      })
      .finally(() => {
        if (!canceled) {
          setLoading(false);
        }
      });

    return () => {
      canceled = true;
    };
  }, [open, presetItemId]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!selected) {
      setError("Select something to share.");
      return;
    }

    setBusy(true);
    setError("");
    try {
      const share = await api.createShare({
        expiresInHours: expiryHours,
        ...(scope === "file"
          ? { torrentFileId: selected.primaryFileId }
          : { torrentId: selected.torrentId }),
      });
      setCreated(share);
      onCreated?.(share);
      await navigator.clipboard?.writeText(share.url).catch(() => undefined);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to create the share link.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal
      title="Create share link"
      open={open}
      onClose={onClose}
      footer={
        <>
          <Button type="button" onClick={onClose} disabled={busy}>
            {created ? "Done" : "Cancel"}
          </Button>
          <Button
            variant="primary"
            type="submit"
            form="create-share-form"
            disabled={busy || loading || !selected || Boolean(created)}
          >
            {busy ? "Creating..." : created ? "Created" : "Generate link"}
          </Button>
        </>
      }
    >
      <form id="create-share-form" className="contents" onSubmit={submit}>
        {error ? <div className="alert alert-error is-visible">{error}</div> : null}

        {!presetItemId ? (
          <Field label="Video">
            <Select
              value={catalogId}
              onChange={(event) => {
                setCatalogId(event.target.value);
                const next = targets.find((target) => target.catalogId === event.target.value);
                setScope(next && next.fileCount > 1 ? "torrent" : "file");
              }}
              disabled={loading || targets.length === 0}
            >
              {targets.length === 0 ? <option value="">No videos ready to share</option> : null}
              {targets.map((target) => (
                <option key={target.catalogId} value={target.catalogId}>
                  {target.title}
                </option>
              ))}
            </Select>
          </Field>
        ) : (
          <Field label="Video">
            <Input readOnly value={selected?.title || (loading ? "Loading…" : "Unavailable")} />
          </Field>
        )}

        <Field label="Share scope">
          <Select value={scope} onChange={(event) => setScope(event.target.value as api.ShareScope)}>
            <option value="file">Single video</option>
            <option value="torrent">
              {selected && selected.fileCount > 1 ? `Full torrent (${selected.fileCount} files)` : "Full torrent"}
            </option>
          </Select>
        </Field>

        <Field label="Expiration">
          <Select value={String(expiryHours)} onChange={(event) => setExpiryHours(Number(event.target.value))}>
            {EXPIRY_OPTIONS.map((option) => (
              <option key={option.hours} value={option.hours}>
                {option.label}
              </option>
            ))}
          </Select>
        </Field>

        {created ? (
          <>
            <div className="alert alert-success is-visible">Share link created and copied to clipboard.</div>
            <div className="share-link">
              <Input readOnly value={created.url} />
              <Button type="button" onClick={() => void navigator.clipboard?.writeText(created.url)}>
                Copy
              </Button>
            </div>
          </>
        ) : null}
      </form>
    </Modal>
  );
}
