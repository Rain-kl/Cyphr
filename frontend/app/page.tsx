// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import { usePathname, useRouter } from 'next/navigation';
import { useEffect } from 'react';

export default function RootPage() {
  const pathname = usePathname();
  const router = useRouter();

  useEffect(() => {
    if (pathname === '/' || pathname === '') {
      router.replace('/asr');
    }
  }, [pathname, router]);

  return null;
}
