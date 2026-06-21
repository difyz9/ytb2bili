export interface TimelineSubtitle {
  index: number;
  from: number;
  to: number;
  content: string;
}

function parseTimestamp(rawValue: string): number | null {
  const normalized = rawValue.trim().replace(',', '.');
  const parts = normalized.split(':');
  if (parts.length < 2 || parts.length > 3) return null;

  const [hoursPart, minutesPart, secondsPart] =
    parts.length === 3 ? parts : ['0', parts[0], parts[1]];

  const hours = Number(hoursPart);
  const minutes = Number(minutesPart);
  const seconds = Number(secondsPart);
  if (!Number.isFinite(hours) || !Number.isFinite(minutes) || !Number.isFinite(seconds)) {
    return null;
  }

  return hours * 3600 + minutes * 60 + seconds;
}

export function parseSubtitleTimeline(source: string): TimelineSubtitle[] {
  const blocks = source
    .replace(/\r\n/g, '\n')
    .replace(/\r/g, '\n')
    .split(/\n\s*\n/g)
    .map((block) => block.trim())
    .filter(Boolean);

  const subtitles: TimelineSubtitle[] = [];

  for (const block of blocks) {
    const lines = block
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean);

    const timelineLineIndex = lines.findIndex((line) => line.includes('-->'));
    if (timelineLineIndex === -1) continue;

    const [fromRaw, toRaw] = lines[timelineLineIndex].split('-->').map((part) => part.trim());
    const from = parseTimestamp(fromRaw);
    const to = parseTimestamp(toRaw.split(' ')[0]);
    if (from == null || to == null || to <= from) continue;

    const content = lines.slice(timelineLineIndex + 1).join('\n').trim();
    if (!content) continue;

    subtitles.push({
      index: subtitles.length,
      from,
      to,
      content,
    });
  }

  return subtitles;
}

export function subtitleTextToVttUrl(source: string): string {
  const trimmed = source.trimStart();
  const vttText = trimmed.startsWith('WEBVTT')
    ? source
    : `WEBVTT\n\n${source.replace(/(\d{2}:\d{2}:\d{2}),(\d{3})/g, '$1.$2')}`;

  return URL.createObjectURL(new Blob([vttText], { type: 'text/vtt' }));
}