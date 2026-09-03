// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

import type { InternalAxiosRequestConfig } from 'axios';
import apiClient from '../core/api-client';
import { BaseService } from '../core/base.service';
import type { ApiResponse } from '../core/types';
import type {
  JobDTO,
  JobListDTO,
  ListJobsParams,
  LogMessage,
  ModelDTO,
} from './types';

export class TranscribeService extends BaseService {
  protected static readonly basePath = '/api/v1';

  /**
   * Submit audio file for transcription with X-Async: true
   */
  static async submitTranscription(
    formData: FormData,
  ): Promise<{ job_id: number; status: string }> {
    const config = {
      headers: {
        'Content-Type': 'multipart/form-data',
        'X-Async': 'true',
      },
    } as unknown as InternalAxiosRequestConfig;

    const response = await apiClient.post<
      ApiResponse<{ job_id: number; status: string }>
    >('/api/v1/audio/transcriptions', formData, config);
    return response.data.data;
  }

  /**
   * List jobs for current user
   */
  static async listMyJobs(params?: ListJobsParams): Promise<JobListDTO> {
    return this.get<JobListDTO>('/jobs', params as Record<string, unknown>);
  }

  /**
   * Get job detail by ID
   */
  static async getJobDetail(id: number | string): Promise<JobDTO> {
    return this.get<JobDTO>(`/jobs/${id}`);
  }

  /**
   * List active transcription models
   */
  static async listModels(keyword?: string): Promise<ModelDTO[]> {
    return this.get<ModelDTO[]>('/models', keyword ? { keyword } : undefined);
  }

  /**
   * Connect to SSE log stream for a job
   */
  static streamJobLogs(
    id: number | string,
    onLog: (log: LogMessage) => void,
    onFinish: (job?: JobDTO) => void,
    onError?: (err: Event) => void,
  ): () => void {
    const url = `/api/v1/jobs/${id}/stream`;
    const eventSource = new EventSource(url);

    eventSource.addEventListener('log', (event: MessageEvent) => {
      try {
        const data = JSON.parse(event.data) as LogMessage;
        onLog(data);
      } catch (err) {
        console.error('[SSE] Failed to parse log event:', err);
      }
    });

    eventSource.addEventListener('finish', (event: MessageEvent) => {
      try {
        let job: JobDTO | undefined;
        if (event.data) {
          job = JSON.parse(event.data) as JobDTO;
        }
        onFinish(job);
      } catch (err) {
        console.error('[SSE] Failed to parse finish event:', err);
        onFinish();
      } finally {
        eventSource.close();
      }
    });

    eventSource.onerror = (err: Event) => {
      if (onError) onError(err);
      eventSource.close();
    };

    return () => {
      eventSource.close();
    };
  }
}
