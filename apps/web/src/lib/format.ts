export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return bytes === 0 ? "Unlimited" : "0 B";
  }

  return formatByteValue(bytes);
}

export function formatTransferRate(bytesPerSecond: number): string {
  if (!Number.isFinite(bytesPerSecond) || bytesPerSecond <= 0) {
    return "0 B/s";
  }

  return `${formatByteValue(bytesPerSecond)}/s`;
}

function formatByteValue(bytes: number): string {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }

  const digits = value >= 10 || unit === 0 ? 0 : 1;
  return `${value.toFixed(digits)} ${units[unit]}`;
}

export function formatDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toISOString().slice(0, 10);
}

export function toBytes(value: string): number {
  const trimmed = value.trim();
  if (!trimmed || /unlimited/i.test(trimmed)) {
    return 0;
  }

  const match = trimmed.match(/^(\d+(?:\.\d+)?)\s*(gb|tb|mb|kb|b)?$/i);
  if (!match) {
    return Number.NaN;
  }

  const amount = Number(match[1]);
  const unit = (match[2] || "GB").toLowerCase();
  const scale: Record<string, number> = {
    b: 1,
    kb: 1024,
    mb: 1024 ** 2,
    gb: 1024 ** 3,
    tb: 1024 ** 4,
  };

  return Math.round(amount * scale[unit]);
}
