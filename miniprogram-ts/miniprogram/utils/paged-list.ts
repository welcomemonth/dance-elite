import type { PagedList } from "../types/api";

export interface PageRequest {
  page: number;
  page_size: number;
}

export type PagedFetcher<T, Q extends object> = (params: PageRequest & Q) => Promise<PagedList<T>>;

export interface PagedListState<T> {
  list: T[];
  page: number;
  pageSize: number;
  total: number;
  /** 上拉加载中 */
  loading: boolean;
  /** 下拉刷新中 */
  refreshing: boolean;
  finished: boolean;
  error: string | null;
}

export interface PagedListOptions<T, Q extends object> {
  fetcher: PagedFetcher<T, Q>;
  /** 固定查询条件（如赛事状态筛选） */
  query?: Q;
  pageSize?: number;
}

export type PagedListListener<T> = (state: PagedListState<T>) => void;

/**
 * 分页列表控制器：封装下拉刷新、上拉加载、防重复请求与请求竞态。
 *
 * 用法：
 *   const list = new PagedListController<Item, { status?: string }>({ fetcher: http.get... });
 *   list.subscribe((state) => this.setData(state));
 *   list.refresh(); / list.loadMore(); / list.setQuery({ status: "registering" });
 */
export class PagedListController<T, Q extends object = Record<string, never>> {
  private readonly fetcher: PagedFetcher<T, Q>;
  private readonly pageSize: number;
  private query: Q;
  private readonly listeners = new Set<PagedListListener<T>>();
  private readonly state: PagedListState<T>;
  private requestSeq = 0;

  constructor(options: PagedListOptions<T, Q>) {
    this.fetcher = options.fetcher;
    this.pageSize = options.pageSize ?? 20;
    this.query = options.query ?? ({} as Q);
    this.state = {
      list: [],
      page: 0,
      pageSize: this.pageSize,
      total: 0,
      loading: false,
      refreshing: false,
      finished: false,
      error: null,
    };
  }

  get snapshot(): PagedListState<T> {
    return { ...this.state, list: [...this.state.list] };
  }

  subscribe(listener: PagedListListener<T>): () => void {
    this.listeners.add(listener);
    listener(this.snapshot);
    return () => {
      this.listeners.delete(listener);
    };
  }

  /** 更新查询条件并重新从第一页加载。 */
  setQuery(patch: Partial<Q>): Promise<void> {
    this.query = { ...this.query, ...patch };
    return this.refresh();
  }

  /** 清空列表（不请求）。 */
  clear(): void {
    this.state.list = [];
    this.state.page = 0;
    this.state.total = 0;
    this.state.finished = false;
    this.state.error = null;
    this.emit();
  }

  /** 下拉刷新：回到第一页并替换列表。 */
  refresh(): Promise<void> {
    if (this.state.refreshing) {
      return Promise.resolve();
    }
    this.state.refreshing = true;
    this.state.error = null;
    this.emit();

    const seq = ++this.requestSeq;
    return this.fetchPage(1, true, seq).finally(() => {
      this.state.refreshing = false;
      this.emit();
    });
  }

  /** 上拉加载下一页；已加载完或正在请求时直接返回。 */
  loadMore(): Promise<void> {
    if (this.state.refreshing || this.state.loading || this.state.finished) {
      return Promise.resolve();
    }
    this.state.loading = true;
    this.state.error = null;
    this.emit();

    const seq = ++this.requestSeq;
    return this.fetchPage(this.state.page + 1, false, seq).finally(() => {
      this.state.loading = false;
      this.emit();
    });
  }

  private async fetchPage(page: number, replace: boolean, seq: number): Promise<void> {
    try {
      const result = await this.fetcher({ ...this.query, page, page_size: this.pageSize });
      if (seq !== this.requestSeq) {
        return;
      }

      const list = replace ? result.list : [...this.state.list, ...result.list];
      this.state.list = list;
      this.state.page = page;
      this.state.total = result.total;
      this.state.finished = list.length >= result.total || result.list.length < this.pageSize;
      this.state.error = null;
      this.emit();
    } catch (error) {
      if (seq !== this.requestSeq) {
        return;
      }
      this.state.error = error instanceof Error ? error.message : String(error);
      this.emit();
      throw error;
    }
  }

  private emit(): void {
    const snapshot = this.snapshot;
    this.listeners.forEach((listener) => listener(snapshot));
  }
}
