'use client';

import { useCallback, useMemo } from 'react';
import { useRouter, useSearchParams, usePathname } from 'next/navigation';

export interface UseListUrlParamsOptions<TStatus extends string = string> {
  defaultPage?: number;
  defaultLimit?: number;
  defaultSearch?: string;
  defaultStatus?: TStatus;
  paramNames?: {
    page?: string;
    limit?: string;
    search?: string;
    status?: string;
  };
}

export interface UseListUrlParamsReturn<TStatus extends string = string> {
  page: number;
  limit: number;
  search: string;
  status: TStatus;
  setPage: (page: number) => void;
  setLimit: (limit: number) => void;
  setSearch: (search: string) => void;
  setStatus: (status: TStatus) => void;
  setFilters: (filters: {
    page?: number;
    limit?: number;
    search?: string;
    status?: TStatus;
  }) => void;
  queryString: string;
  buildUrl: (
    basePath: string,
    overrides?: {
      page?: number;
      limit?: number;
      search?: string;
      status?: TStatus;
    }
  ) => string;
}

export function useListUrlParams<TStatus extends string = string>(
  options: UseListUrlParamsOptions<TStatus> = {}
): UseListUrlParamsReturn<TStatus> {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const {
    defaultPage = 1,
    defaultLimit = 10,
    defaultSearch = '',
    defaultStatus = 'all' as TStatus,
    paramNames = {},
  } = options;

  const pageKey = paramNames.page || 'page';
  const limitKey = paramNames.limit || 'limit';
  const searchKey = paramNames.search || 'q';
  const statusKey = paramNames.status || 'status';

  // Read current values from URL query string
  const page = useMemo(() => {
    const raw = searchParams.get(pageKey);
    if (!raw) return defaultPage;
    const parsed = parseInt(raw, 10);
    return Number.isFinite(parsed) && parsed > 0 ? parsed : defaultPage;
  }, [searchParams, pageKey, defaultPage]);

  const limit = useMemo(() => {
    const raw = searchParams.get(limitKey);
    if (!raw) return defaultLimit;
    const parsed = parseInt(raw, 10);
    return Number.isFinite(parsed) && parsed > 0 ? parsed : defaultLimit;
  }, [searchParams, limitKey, defaultLimit]);

  const search = useMemo(() => {
    return searchParams.get(searchKey) || defaultSearch;
  }, [searchParams, searchKey, defaultSearch]);

  const status = useMemo(() => {
    return (searchParams.get(statusKey) as TStatus) || defaultStatus;
  }, [searchParams, statusKey, defaultStatus]);

  // Helper to compute query string
  const getQueryString = useCallback(
    (overrides: {
      page?: number;
      limit?: number;
      search?: string;
      status?: TStatus;
    } = {}) => {
      const params = new URLSearchParams(searchParams.toString());

      const nextPage = overrides.page !== undefined ? overrides.page : page;
      const nextLimit = overrides.limit !== undefined ? overrides.limit : limit;
      const nextSearch = overrides.search !== undefined ? overrides.search : search;
      const nextStatus = overrides.status !== undefined ? overrides.status : status;

      // Handle page
      if (nextPage <= 1) params.delete(pageKey);
      else params.set(pageKey, String(nextPage));

      // Handle limit
      if (nextLimit === defaultLimit) params.delete(limitKey);
      else params.set(limitKey, String(nextLimit));

      // Handle search
      if (!nextSearch) params.delete(searchKey);
      else params.set(searchKey, nextSearch);

      // Handle status
      if (nextStatus === defaultStatus) params.delete(statusKey);
      else params.set(statusKey, nextStatus);

      return params.toString();
    },
    [
      searchParams,
      page,
      limit,
      search,
      status,
      pageKey,
      limitKey,
      searchKey,
      statusKey,
      defaultLimit,
      defaultStatus,
    ]
  );

  const queryString = useMemo(() => getQueryString(), [getQueryString]);

  const updateUrl = useCallback(
    (overrides: {
      page?: number;
      limit?: number;
      search?: string;
      status?: TStatus;
    }) => {
      const qs = getQueryString(overrides);
      const target = qs ? `${pathname}?${qs}` : pathname;
      router.replace(target, { scroll: false });
    },
    [getQueryString, pathname, router]
  );

  const setPage = useCallback(
    (newPage: number) => {
      updateUrl({ page: newPage });
    },
    [updateUrl]
  );

  const setLimit = useCallback(
    (newLimit: number) => {
      // Reset to page 1 when changing page size
      updateUrl({ limit: newLimit, page: 1 });
    },
    [updateUrl]
  );

  const setSearch = useCallback(
    (newSearch: string) => {
      // Reset to page 1 when search changes
      updateUrl({ search: newSearch, page: 1 });
    },
    [updateUrl]
  );

  const setStatus = useCallback(
    (newStatus: TStatus) => {
      // Reset to page 1 when status filter changes
      updateUrl({ status: newStatus, page: 1 });
    },
    [updateUrl]
  );

  const setFilters = useCallback(
    (filters: {
      page?: number;
      limit?: number;
      search?: string;
      status?: TStatus;
    }) => {
      updateUrl(filters);
    },
    [updateUrl]
  );

  const buildUrl = useCallback(
    (
      basePath: string,
      overrides: {
        page?: number;
        limit?: number;
        search?: string;
        status?: TStatus;
      } = {}
    ) => {
      const qs = getQueryString(overrides);
      return qs ? `${basePath}?${qs}` : basePath;
    },
    [getQueryString]
  );

  return {
    page,
    limit,
    search,
    status,
    setPage,
    setLimit,
    setSearch,
    setStatus,
    setFilters,
    queryString,
    buildUrl,
  };
}
