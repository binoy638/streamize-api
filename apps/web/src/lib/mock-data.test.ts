import { describe, expect, it } from "vitest";

import { filesForTorrent, findMedia, findTorrent, mediaItems, torrents } from "./mock-data";

describe("prototype mock data", () => {
  it("contains routable media and torrent records", () => {
    expect(mediaItems.length).toBeGreaterThan(0);
    expect(torrents.length).toBeGreaterThan(0);
    expect(findMedia(mediaItems[0].id).id).toBe(mediaItems[0].id);
    expect(findTorrent(torrents[0].id).id).toBe(torrents[0].id);
  });

  it("keeps torrent files addressable by torrent id", () => {
    expect(filesForTorrent("tor_neon_harbor").map((file) => file.torrentId)).toEqual([
      "tor_neon_harbor",
      "tor_neon_harbor",
    ]);
  });
});
