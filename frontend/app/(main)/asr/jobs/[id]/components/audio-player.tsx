// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { Button } from '@/components/ui/button';
import { Slider } from '@/components/ui/slider';
import {
  FastForward,
  Pause,
  Play,
  Rewind,
  Volume2,
  VolumeX,
} from 'lucide-react';

export interface AudioPlayerRef {
  seekTo: (seconds: number) => void;
  play: () => void;
  pause: () => void;
}

interface AudioPlayerProps {
  jobId: string | number;
  mediaUrl?: string;
  audioStoragePath?: string;
  // Fallback duration from job detail (transcription duration) so the slider
  // stays usable before audio metadata loads.
  initialDuration?: number;
  onTimeUpdate?: (currentTime: number) => void;
}

export const AudioPlayer = React.forwardRef<AudioPlayerRef, AudioPlayerProps>(
  function AudioPlayer(
    { mediaUrl, audioStoragePath, initialDuration, onTimeUpdate },
    ref,
  ) {
    const audioRef = React.useRef<HTMLAudioElement | null>(null);
    const [isPlaying, setIsPlaying] = React.useState(false);
    const [currentTime, setCurrentTime] = React.useState(0);
    const [duration, setDuration] = React.useState(0);
    const [volume, setVolume] = React.useState(1);
    const [isMuted, setIsMuted] = React.useState(false);
    const [isSeeking, setIsSeeking] = React.useState(false);
    const [seekPreview, setSeekPreview] = React.useState(0);
    const [audioError, setAudioError] = React.useState(false);
    const isSeekingRef = React.useRef(false);
    // Seek requested before metadata is ready; applied on loadedmetadata.
    const pendingSeekRef = React.useRef<number | null>(null);
    const onTimeUpdateRef = React.useRef(onTimeUpdate);
    onTimeUpdateRef.current = onTimeUpdate;

    const resolvedMediaUrl = React.useMemo(() => {
      if (mediaUrl) return mediaUrl;
      if (audioStoragePath) {
        const match = audioStoragePath.match(/(\d+)\.[^.]+$/);
        if (match?.[1]) {
          return `/f/${match[1]}`;
        }
      }
      return '';
    }, [mediaUrl, audioStoragePath]);

    const applySeek = React.useCallback((seconds: number) => {
      const el = audioRef.current;
      const target = Number.isFinite(seconds) ? Math.max(0, seconds) : 0;
      // Sync UI immediately so the active SRT cue follows without waiting
      // for the next timeupdate event.
      setCurrentTime(target);
      onTimeUpdateRef.current?.(target);
      if (!el) {
        pendingSeekRef.current = target;
        return;
      }
      // Metadata not ready yet: setting currentTime is ignored by browsers,
      // so defer the seek until loadedmetadata fires.
      if (el.readyState < 1 || !Number.isFinite(el.duration)) {
        pendingSeekRef.current = target;
        // Trigger metadata load for the deferred seek.
        el.preload = 'auto';
        try {
          el.load();
        } catch {
          // Ignore: loadedmetadata handler will apply the pending seek.
        }
        el.play().catch(() => {});
        setIsPlaying(true);
        return;
      }
      try {
        el.currentTime = target;
      } catch {
        pendingSeekRef.current = target;
      }
      el.play().catch(() => {});
      setIsPlaying(true);
    }, []);

    const flushPendingSeek = React.useCallback(() => {
      const el = audioRef.current;
      const pending = pendingSeekRef.current;
      if (el && pending !== null && el.readyState >= 1) {
        try {
          el.currentTime = pending;
          pendingSeekRef.current = null;
        } catch {
          // Keep pending; retry on next metadata/canplay event.
        }
      }
    }, []);

    React.useImperativeHandle(ref, () => ({
      seekTo: (seconds: number) => {
        applySeek(seconds);
      },
      play: () => {
        audioRef.current?.play().catch(() => {});
        setIsPlaying(true);
      },
      pause: () => {
        audioRef.current?.pause();
        setIsPlaying(false);
      },
    }));

    const togglePlay = () => {
      if (!audioRef.current) return;
      if (isPlaying) {
        audioRef.current.pause();
        setIsPlaying(false);
      } else {
        audioRef.current.play().catch(() => {});
        setIsPlaying(true);
      }
    };

    const skip = (seconds: number) => {
      if (!audioRef.current) return;
      const upper =
        Number.isFinite(duration) && duration > 0
          ? duration
          : Number.isFinite(initialDuration) && (initialDuration ?? 0) > 0
            ? (initialDuration ?? 0)
            : Number.POSITIVE_INFINITY;
      const target = Math.max(
        0,
        Math.min(upper, audioRef.current.currentTime + seconds),
      );
      applySeek(target);
    };

    const elementDurationReady = Number.isFinite(duration) && duration > 0;
    const fallbackDuration =
      Number.isFinite(initialDuration) && (initialDuration ?? 0) > 0
        ? (initialDuration ?? 0)
        : 0;
    const effectiveDuration = elementDurationReady
      ? duration
      : fallbackDuration;
    // The slider stays draggable as long as we know any duration (element or
    // job fallback). Actual element seek is deferred until metadata is ready.
    const canSeek = Boolean(resolvedMediaUrl) && effectiveDuration > 0;

    const handleSeekPreview = (value: number[]) => {
      isSeekingRef.current = true;
      setIsSeeking(true);
      setSeekPreview(value[0] ?? 0);
    };

    const handleSeekCommit = (value: number[]) => {
      const target = value[0] ?? 0;
      isSeekingRef.current = false;
      setIsSeeking(false);
      if (!canSeek) return;
      applySeek(target);
    };

    const handleVolumeChange = (value: number[]) => {
      if (!audioRef.current) return;
      const newVol = value[0];
      audioRef.current.volume = newVol;
      setVolume(newVol);
      setIsMuted(newVol === 0);
    };

    const toggleMute = () => {
      if (!audioRef.current) return;
      if (isMuted) {
        audioRef.current.muted = false;
        audioRef.current.volume = volume || 1;
        setIsMuted(false);
      } else {
        audioRef.current.muted = true;
        setIsMuted(true);
      }
    };

    const formatTime = (seconds: number) => {
      const m = Math.floor(seconds / 60);
      const s = Math.floor(seconds % 60);
      return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
    };

    return (
      <div className='border border-dashed shadow-none rounded-lg bg-background p-4'>
        <audio
          ref={audioRef}
          src={resolvedMediaUrl}
          preload='auto'
          onTimeUpdate={() => {
            if (audioRef.current && !isSeekingRef.current) {
              const curr = audioRef.current.currentTime;
              setCurrentTime(curr);
              onTimeUpdateRef.current?.(curr);
            }
          }}
          onLoadedMetadata={() => {
            if (audioRef.current) {
              const d = audioRef.current.duration;
              setDuration(Number.isFinite(d) ? d || 0 : 0);
              setAudioError(false);
            }
            flushPendingSeek();
          }}
          onDurationChange={() => {
            if (audioRef.current) {
              const d = audioRef.current.duration;
              if (Number.isFinite(d) && d > 0) setDuration(d);
            }
          }}
          onCanPlay={() => {
            flushPendingSeek();
          }}
          onError={() => {
            setAudioError(true);
            setIsPlaying(false);
          }}
          onEnded={() => setIsPlaying(false)}
        />

        <div className='space-y-3'>
          {/* Progress Slider */}
          <div className='space-y-1'>
            <Slider
              value={[isSeeking ? seekPreview : currentTime]}
              min={0}
              max={canSeek ? effectiveDuration : 100}
              step={0.1}
              disabled={!canSeek}
              onValueChange={handleSeekPreview}
              onValueCommit={handleSeekCommit}
              className='cursor-pointer'
              aria-label='Audio Seek'
            />
            <div className='flex justify-between text-[11px] font-mono text-muted-foreground'>
              <span>{formatTime(isSeeking ? seekPreview : currentTime)}</span>
              <span>{formatTime(effectiveDuration)}</span>
            </div>
            {!resolvedMediaUrl ? (
              <p className='text-[11px] text-muted-foreground'>
                音频地址缺失，无法播放与跳转（media_url / audio_storage_path
                为空）。
              </p>
            ) : audioError ? (
              <p className='text-[11px] text-rose-500'>
                音频加载失败（/f/ 接口可能返回
                401/404），请检查登录态后刷新重试；时间片高亮仍会跟随拖动位置。
              </p>
            ) : null}
          </div>

          {/* Controls Bar */}
          <div className='flex items-center justify-between'>
            {/* Playback Transport Buttons */}
            <div className='flex items-center gap-1.5'>
              <Button
                variant='ghost'
                size='icon'
                className='size-8'
                onClick={() => skip(-5)}
                aria-label='Rewind 5 seconds'
              >
                <Rewind className='size-4' />
              </Button>

              <Button
                size='icon'
                className='size-9 rounded-full'
                onClick={togglePlay}
                aria-label={isPlaying ? 'Pause' : 'Play'}
              >
                {isPlaying ? (
                  <Pause className='size-4' />
                ) : (
                  <Play className='size-4 ml-0.5' />
                )}
              </Button>

              <Button
                variant='ghost'
                size='icon'
                className='size-8'
                onClick={() => skip(5)}
                aria-label='Forward 5 seconds'
              >
                <FastForward className='size-4' />
              </Button>
            </div>

            {/* Volume Control */}
            <div className='flex items-center gap-2'>
              <Button
                variant='ghost'
                size='icon'
                className='size-8'
                onClick={toggleMute}
                aria-label={isMuted ? 'Unmute' : 'Mute'}
              >
                {isMuted || volume === 0 ? (
                  <VolumeX className='size-4 text-muted-foreground' />
                ) : (
                  <Volume2 className='size-4' />
                )}
              </Button>
              <Slider
                value={[isMuted ? 0 : volume]}
                min={0}
                max={1}
                step={0.05}
                onValueChange={handleVolumeChange}
                className='w-20 cursor-pointer'
                aria-label='Volume Slider'
              />
            </div>
          </div>
        </div>
      </div>
    );
  },
);
