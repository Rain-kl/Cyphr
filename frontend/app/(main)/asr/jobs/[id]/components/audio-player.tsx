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
  jobId: number;
  onTimeUpdate?: (currentTime: number) => void;
}

export const AudioPlayer = React.forwardRef<AudioPlayerRef, AudioPlayerProps>(
  function AudioPlayer({ jobId, onTimeUpdate }, ref) {
    const audioRef = React.useRef<HTMLAudioElement | null>(null);
    const [isPlaying, setIsPlaying] = React.useState(false);
    const [currentTime, setCurrentTime] = React.useState(0);
    const [duration, setDuration] = React.useState(0);
    const [volume, setVolume] = React.useState(1);
    const [isMuted, setIsMuted] = React.useState(false);

    const mediaUrl = `/api/v1/agent/jobs/${jobId}/media`;

    React.useImperativeHandle(ref, () => ({
      seekTo: (seconds: number) => {
        if (audioRef.current) {
          audioRef.current.currentTime = seconds;
          audioRef.current.play().catch(() => {});
          setIsPlaying(true);
        }
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
      const target = Math.max(
        0,
        Math.min(duration, audioRef.current.currentTime + seconds),
      );
      audioRef.current.currentTime = target;
      setCurrentTime(target);
    };

    const handleSeek = (value: number[]) => {
      if (!audioRef.current) return;
      const target = value[0];
      audioRef.current.currentTime = target;
      setCurrentTime(target);
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
      <div className='rounded-xl border bg-card p-4 shadow-sm'>
        <audio
          ref={audioRef}
          src={mediaUrl}
          onTimeUpdate={() => {
            if (audioRef.current) {
              const curr = audioRef.current.currentTime;
              setCurrentTime(curr);
              if (onTimeUpdate) onTimeUpdate(curr);
            }
          }}
          onLoadedMetadata={() => {
            if (audioRef.current) {
              setDuration(audioRef.current.duration || 0);
            }
          }}
          onEnded={() => setIsPlaying(false)}
        />

        <div className='space-y-3'>
          {/* Progress Slider */}
          <div className='space-y-1'>
            <Slider
              value={[currentTime]}
              min={0}
              max={duration || 100}
              step={0.1}
              onValueChange={handleSeek}
              className='cursor-pointer'
              aria-label='Audio Seek'
            />
            <div className='flex justify-between text-[11px] font-mono text-muted-foreground'>
              <span>{formatTime(currentTime)}</span>
              <span>{formatTime(duration)}</span>
            </div>
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
