'use client';

import { useState } from 'react';
import { AppSidebar } from '@/components/layout/sidebar';
import { SiteHeader } from '@/components/layout/header';
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar';
import { useAuthRedirect } from '@/hooks/use-auth-redirect';

export default function MainLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const [isFullWidth, setIsFullWidth] = useState(false);

  useAuthRedirect();

  return (
    <SidebarProvider
      className='h-screen'
      style={
        {
          '--header-height': '60px',
        } as React.CSSProperties
      }
    >
      <AppSidebar />
      <SidebarInset className='flex flex-col min-w-0 h-screen'>
        <SiteHeader
          isFullWidth={isFullWidth}
          onToggleFullWidth={setIsFullWidth}
        />
        <div className='flex flex-1 flex-col bg-background overflow-y-auto overflow-x-hidden min-w-0 hide-scrollbar'>
          <div
            className={`w-full mx-auto px-4 sm:px-6 md:px-8 lg:px-12 min-w-0 transition-all duration-300 ease-in-out ${!isFullWidth ? 'max-w-[1320px]' : 'max-w-full'}`}
          >
            {children}
          </div>
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}
