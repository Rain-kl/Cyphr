// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

import { NodeDetailClient } from './node-detail-client';

export function generateStaticParams() {
  return [{ id: '0' }];
}

export default function NodeDetailPage() {
  return <NodeDetailClient />;
}
