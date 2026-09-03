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
  token_prefix: string;
  is_active: boolean;
  is_online?: boolean;
  loaded_models?: string[];
  running_jobs?: number;
  system?: SystemStatsDTO;
  last_ip?: string;
  last_seen_at?: string;
  created_at: string;
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

export interface JobDTO {
  id: string | number;
  user_id: string | number;
  node_id?: string | number;
  original_file_name: string;
  audio_storage_path?: string;
  media_url?: string;
  model: string;
  task_type: string;
  language?: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
  progress: number;
  duration?: number;
  result_text?: string;
  result_json?: string;
  error_msg?: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
}

export interface JobListDTO {
  items: JobDTO[];
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
