import * as React from 'react';
import { AlertCircle } from 'lucide-react';
import { Button } from '@/components/ui/button';

import { cn } from '@/lib/utils';
import { useTranslations } from 'next-intl';

/**
 * 错误展示组件属性
 * 用于统一显示加载失败、请求失败等错误状态
 */
export interface ErrorDisplayProps {
  title?: string;
  message?: string;
  error?: Error | null;
  onRetry?: () => void;
  retryText?: string;
  icon?: React.ComponentType<{ className?: string }>;
  className?: string;
  showStack?: boolean;
}

/**
 * 错误展示组件
 * 用于统一显示加载失败、请求失败等错误状态
 *
 * @example
 * ```tsx
 * // 基础用法
 * <ErrorDisplay
 *   title="加载失败"
 *   message="无法获取数据，请稍后重试"
 * />
 *
 * // 带重试按钮
 * <ErrorDisplay
 *   error={error}
 *   onRetry={() => refetch()}
 * />
 *
 * // 自定义样式
 * <ErrorDisplay
 *   title="网络错误"
 *   message="请检查您的网络连接"
 *   className="min-h-[400px]"
 * />
 * ```
 * @param {ErrorDisplayProps} props - 错误展示组件属性
 * @returns {React.ReactNode} 错误展示组件
 */
export function ErrorDisplay({
  title,
  message,
  error,
  onRetry,
  retryText,
  icon: Icon = AlertCircle,
  className,
  showStack = false,
}: ErrorDisplayProps) {
  const t = useTranslations('layout.error');
  const displayTitle = title ?? t('loadFailed');
  const displayRetryText = retryText ?? t('retry');
  const errorMessage = message || error?.message || t('unknownError');

  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center py-12 text-center',
        className,
      )}
    >
      <div className='size-12 rounded-full bg-red-100 dark:bg-red-900/20 flex items-center justify-center mb-4'>
        <Icon className='size-6 text-red-600 dark:text-red-400' />
      </div>

      <p className='text-lg font-semibold mb-2'>{displayTitle}</p>

      <p className='text-sm text-muted-foreground max-w-md mb-4'>
        {errorMessage}
      </p>

      {onRetry && (
        <Button onClick={onRetry} variant='outline' className='mt-2'>
          {displayRetryText}
        </Button>
      )}

      {showStack && error?.stack && process.env.NODE_ENV === 'development' && (
        <details className='mt-6 text-left max-w-2xl w-full'>
          <summary className='cursor-pointer text-xs text-muted-foreground hover:text-foreground'>
            {t('viewDetails')}
          </summary>
          <pre className='mt-2 p-4 bg-muted rounded-lg text-xs overflow-auto'>
            {error.stack}
          </pre>
        </details>
      )}
    </div>
  );
}

/**
 * 区域级错误组件
 * 适合在较小的区域显示错误
 *
 * @example
 * ```tsx
 * <ErrorInline
 *   message="发生错误"
 *   error={error}
 *   onRetry={() => refetch()}
 * />
 * ```
 */
export function ErrorInline({
  message,
  error,
  onRetry,
  className,
}: Omit<ErrorDisplayProps, 'title' | 'icon' | 'showStack'>) {
  const t = useTranslations('layout.error');
  const errorMessage = message || error?.message || t('unknownErrorShort');

  return (
    <div
      className={cn(
        'flex items-center gap-2 text-sm text-red-600 dark:text-red-400',
        className,
      )}
    >
      <AlertCircle className='size-3 shrink-0 text-destructive' />
      <span className='flex-1 text-xs'>{errorMessage}</span>
      {onRetry && (
        <Button
          onClick={onRetry}
          variant='secondary'
          size='sm'
          className='h-6 px-4 text-xs'
        >
          {t('retry')}
        </Button>
      )}
    </div>
  );
}

/**
 * 页面级错误组件
 * 适合作为页面主要内容的错误状态
 *
 * @example
 * ```tsx
 * <ErrorPage
 *   title="加载失败"
 *   message="无法获取数据，请稍后重试"
 *   error={error}
 *   onRetry={() => refetch()}
 * />
 * ```
 */
export function ErrorPage(props: ErrorDisplayProps) {
  return (
    <div className='flex items-center justify-center min-h-[calc(100vh-8rem)]'>
      <ErrorDisplay
        {...props}
        showStack={props.showStack ?? process.env.NODE_ENV === 'development'}
      />
    </div>
  );
}
