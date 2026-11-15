/** 带过期时间的本地缓存（基于 wx storage，自动加命名空间前缀）。 */
interface CacheRecord<T> {
  __cache: 1;
  value: T;
  /** 0 表示永不过期。 */
  expiresAt: number;
}

const PREFIX = "de_cache:";
const MARK = 1;

function fullKey(key: string): string {
  return key.startsWith(PREFIX) ? key : `${PREFIX}${key}`;
}

/**
 * 写入缓存。
 * @param ttlSeconds 过期秒数，0 / 不传表示永不过期
 */
export function cacheSet<T>(key: string, value: T, ttlSeconds = 0): void {
  if (!key) {
    return;
  }
  const record: CacheRecord<T> = {
    __cache: MARK,
    value,
    expiresAt: ttlSeconds > 0 ? Date.now() + ttlSeconds * 1000 : 0,
  };
  try {
    wx.setStorageSync(fullKey(key), record);
  } catch (error) {
    console.warn("[cache] 写入失败", key, error);
  }
}

/** 读取缓存；不存在、数据损坏或已过期时返回 fallback。 */
export function cacheGet<T>(key: string, fallback: T | null = null): T | null {
  if (!key) {
    return fallback;
  }
  const storageKey = fullKey(key);
  const raw: unknown = wx.getStorageSync(storageKey);
  if (!raw || typeof raw !== "object") {
    return fallback;
  }

  const record = raw as Partial<CacheRecord<T>>;
  if (record.__cache !== MARK) {
    return fallback;
  }
  if (record.expiresAt && record.expiresAt <= Date.now()) {
    wx.removeStorageSync(storageKey);
    return fallback;
  }
  return record.value as T;
}

export function cacheRemove(key: string): void {
  wx.removeStorageSync(fullKey(key));
}

/** 清空本模板写入的全部缓存，不影响其他 storage 数据。 */
export function cacheClear(): void {
  try {
    const { keys } = wx.getStorageInfoSync();
    keys.forEach((key) => {
      if (key.startsWith(PREFIX)) {
        wx.removeStorageSync(key);
      }
    });
  } catch (error) {
    console.warn("[cache] 清理失败", error);
  }
}

/** 读缓存，未命中时执行 loader 并回填。 */
export async function cacheGetOrSet<T>(key: string, ttlSeconds: number, loader: () => Promise<T>): Promise<T> {
  const cached = cacheGet<T>(key);
  if (cached !== null) {
    return cached;
  }
  const value = await loader();
  cacheSet(key, value, ttlSeconds);
  return value;
}
