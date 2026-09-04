// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

import { BaseService } from '../core/base.service';
import type {
  JobDTO,
  JobListDTO,
  ListAllJobsParams,
  ModelDTO,
  NodeCreatedDTO,
  NodeDTO,
  UpdateNodeConfigParams,
} from './types';

export class AdminTranscribeService extends BaseService {
  protected static readonly basePath = '/api/v1/controller';

  /**
   * List worker nodes with real-time heartbeat and hardware metrics
   */
  static async listNodes(keyword?: string): Promise<NodeDTO[]> {
    return this.get<NodeDTO[]>('/nodes', keyword ? { keyword } : undefined);
  }

  /**
   * Get single node details
   */
  static async getNode(id: number | string): Promise<NodeDTO> {
    return this.get<NodeDTO>(`/nodes/${id}`);
  }

  /**
   * Create a new worker node and obtain one-time raw agent token
   */
  static async createNode(name: string): Promise<NodeCreatedDTO> {
    return this.post<NodeCreatedDTO>('/nodes', { name });
  }

  /**
   * Delete / deregister a worker node
   */
  static async deleteNode(id: number | string): Promise<void> {
    return this.delete<void>(`/nodes/${id}`);
  }

  /**
   * Command an online worker node to load a model
   */
  static async loadModel(
    nodeId: number | string,
    modelName: string,
  ): Promise<void> {
    return this.post<void>(`/nodes/${nodeId}/load-model`, {
      model_name: modelName,
    });
  }

  /**
   * Command an online worker node to unload a model
   */
  static async unloadModel(
    nodeId: number | string,
    modelName: string,
  ): Promise<void> {
    return this.post<void>(`/nodes/${nodeId}/unload-model`, {
      model_name: modelName,
    });
  }

  /**
   * Update worker node configuration (work mode, auto unload, VRAM estimates, allow auto load)
   */
  static async updateNodeConfig(
    nodeId: number | string,
    params: UpdateNodeConfigParams,
  ): Promise<NodeDTO> {
    return this.put<NodeDTO>(`/nodes/${nodeId}/config`, params);
  }

  /**
   * List all models (active and inactive) for admin
   */
  static async listAllModels(keyword?: string): Promise<ModelDTO[]> {
    return this.get<ModelDTO[]>('/models', keyword ? { keyword } : undefined);
  }

  /**
   * Enable or disable a model globally
   */
  static async toggleModelStatus(
    id: number | string,
    isActive: boolean,
  ): Promise<void> {
    return this.put<void>(`/models/${id}/status`, { is_active: isActive });
  }

  /**
   * List all jobs cross-user with filters for admin
   */
  static async listAllJobs(params?: ListAllJobsParams): Promise<JobListDTO> {
    return this.get<JobListDTO>('/jobs', params as Record<string, unknown>);
  }

  /**
   * Get complete details of a transcription job (includes heavy result payload)
   */
  static async getJobDetail(id: number | string): Promise<JobDTO> {
    return this.get<JobDTO>(`/jobs/${id}`);
  }

  /**
   * Requeue a failed transcription job
   */
  static async retryJob(id: number | string): Promise<void> {
    return this.post<void>(`/jobs/${id}/retry`, {});
  }

  /**
   * Batch delete transcription jobs as admin
   */
  static async deleteJobs(
    ids: (string | number)[],
  ): Promise<{ deleted_count: number }> {
    return this.post<{ deleted_count: number }>('/jobs/batch-delete', {
      job_ids: ids.map((id) => Number(id)),
    });
  }
}
