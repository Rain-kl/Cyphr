// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

export interface ModelDTO {
  id: string | number;
  name: string;
  task_type: string;
  description: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface SystemStatsDTO {
  cpu_percent: number;
  ram_percent?: number;
  ram_used_mb?: number;
  ram_total_mb?: number;
  gpu_percent?: number;
  gpu_memory_used_mb?: number;
  gpu_memory_total_mb?: number;
}

export interface NodeDTO {
  id: string | number;
  name: string;
  agent_token?: string;
  token_prefix: string;
  is_active: boolean;
  is_online?: boolean;
  work_mode?: string;
  supported_modes?: string[];
  current_mode?: string;
  allow_auto_load?: boolean;
  auto_unload_minutes?: number;
  max_concurrent_jobs?: number;
  model_vram_estimates?: Record<string, number>;
  loaded_models?: string[];
  downloaded_models?: string[];
  running_jobs?: number;
  system?: SystemStatsDTO;
  last_ip?: string;
  last_seen_at?: string;
  created_at: string;
}

export interface UpdateNodeConfigParams {
  work_mode?: string;
  allow_auto_load?: boolean;
  auto_unload_minutes?: number;
  max_concurrent_jobs?: number;
  model_vram_estimates?: Record<string, number>;
}

export interface NodeCreatedDTO {
  id: string | number;
  name: string;
  agent_token: string;
  token_prefix: string;
  created_at: string;
}

export interface TranscriptSegment {
  id: number;
  seek?: number;
  start: number;
  end: number;
  text: string;
  tokens?: number[];
  temperature?: number;
  avg_logprob?: number;
  compression_ratio?: number;
  no_speech_prob?: number;
}

export interface VerboseJSONResult {
  task?: string;
  language?: string;
  duration?: number;
  text?: string;
  segments?: TranscriptSegment[];
}

export interface JobSummaryDTO {
  id: string | number;
  user_id: string | number;
  node_id?: string | number;
  original_file_name: string;
  model: string;
  task_type: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
  progress: number;
  duration?: number;
  retry_count?: number;
  started_at?: string;
  completed_at?: string;
  created_at: string;
}

export interface JobDTO extends JobSummaryDTO {
  audio_storage_path?: string;
  media_url?: string;
  language?: string;
  result_text?: string;
  // Backend contract: verbose JSON is returned as `openai_response` object.
  // `result_json` is kept only as a legacy fallback (stringified JSON).
  openai_response?: VerboseJSONResult | string | Record<string, unknown>;
  result_json?: string;
  error_msg?: string;
}

// parseVerboseResult extracts structured verbose_json (with segments) from a
// job, supporting both the current `openai_response` contract (object or
// stringified JSON) and the legacy `result_json` string field.
export function parseVerboseResult(
  job: Pick<JobDTO, 'openai_response' | 'result_json'>,
): VerboseJSONResult | null {
  const candidate = job.openai_response ?? job.result_json;
  if (!candidate) return null;
  try {
    if (typeof candidate === 'string') {
      return JSON.parse(candidate) as VerboseJSONResult;
    }
    return candidate as VerboseJSONResult;
  } catch {
    return null;
  }
}

export function getTranscriptFullText(
  job: Pick<JobDTO, 'result_text' | 'openai_response' | 'result_json'>,
  parsed?: VerboseJSONResult | null,
): string {
  if (job.result_text) return job.result_text;
  const verbose = parsed ?? parseVerboseResult(job);
  return verbose?.text || '';
}

export interface JobListDTO {
  items: JobSummaryDTO[];
  total: number;
  page: number;
  page_size: number;
}

export interface LogMessage {
  timestamp: string;
  message: string;
  progress: number;
}

export interface ListJobsParams {
  page?: number;
  page_size?: number;
  status?: string;
  keyword?: string;
}

export interface ListAllJobsParams extends ListJobsParams {
  node_id?: string | number;
  user_id?: string | number;
}
