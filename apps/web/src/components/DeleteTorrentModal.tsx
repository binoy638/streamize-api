import { type FormEvent, useEffect, useState } from "react";

import type { DeleteTorrentOptions } from "../lib/api";
import { Button, Modal } from "./ui";

export function DeleteTorrentModal({
  open,
  title,
  busy,
  onClose,
  onConfirm,
}: {
  open: boolean;
  title: string;
  busy?: boolean;
  onClose: () => void;
  onConfirm: (options: Required<DeleteTorrentOptions>) => Promise<void> | void;
}) {
  const [deleteFiles, setDeleteFiles] = useState(false);
  const [deleteGenerated, setDeleteGenerated] = useState(true);

  useEffect(() => {
    if (!open) {
      return;
    }
    setDeleteFiles(false);
    setDeleteGenerated(true);
  }, [open]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    await onConfirm({ deleteFiles, deleteGenerated });
  }

  return (
    <Modal
      title="Delete torrent"
      open={open}
      onClose={onClose}
      footer={
        <>
          <Button type="button" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button variant="danger" type="submit" form="delete-torrent-form" disabled={busy}>
            {busy ? "Deleting..." : "Delete"}
          </Button>
        </>
      }
    >
      <form id="delete-torrent-form" className="contents" onSubmit={submit}>
        <div className="alert alert-warn is-visible">Delete {title}?</div>
        <label className="check-row">
          <input
            type="checkbox"
            checked={deleteGenerated}
            onChange={(event) => setDeleteGenerated(event.target.checked)}
          />
          <span>Delete transcoded files, subtitles, and preview thumbnails</span>
        </label>
        <label className="check-row">
          <input
            type="checkbox"
            checked={deleteFiles}
            onChange={(event) => setDeleteFiles(event.target.checked)}
          />
          <span>Delete downloaded files from the media volume</span>
        </label>
      </form>
    </Modal>
  );
}
