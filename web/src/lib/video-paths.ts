export function videoPathToUrl(path: string): string {
  if (!path) return '';

  const downloadsIndex = path.indexOf('downloads/');
  if (downloadsIndex !== -1) {
    return `/static/${path.slice(downloadsIndex + 'downloads/'.length)}`;
  }

  return `/static/${path.replace(/^\.?\//, '')}`;
}

function guessStaticDirectory(path?: string): string | null {
  if (!path) return null;

  const downloadsIndex = path.indexOf('downloads/');
  if (downloadsIndex === -1) return null;

  const relativePath = path.slice(downloadsIndex + 'downloads/'.length);
  const directory = relativePath.replace(/\/[^/]+$/, '');
  return directory ? `/static/${directory}` : null;
}

export function guessSubtitleUrl(video: {
  subtitle_path?: string | null;
  video_path?: string | null;
  video_id: string;
}): string | undefined {
  if (video.subtitle_path) {
    const lowerPath = video.subtitle_path.toLowerCase();
    if (lowerPath.endsWith('.srt') || lowerPath.endsWith('.vtt')) {
      return videoPathToUrl(video.subtitle_path);
    }
  }

  const directory = guessStaticDirectory(video.video_path ?? undefined);
  if (!directory) return undefined;

  return `${directory}/${video.video_id}.srt`;
}

export function guessSubtitleAudioBaseUrl(video: {
  video_path?: string | null;
  video_id: string;
}): string | undefined {
  const directory = guessStaticDirectory(video.video_path ?? undefined);
  if (!directory) return undefined;

  return `${directory}/audio`;
}