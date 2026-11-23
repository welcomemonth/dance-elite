import React, { useEffect, useMemo, useState } from 'react';
import type { LotteryMeta, LotteryStatus } from './service';

/** 服务端时间与本地时钟的偏移（毫秒）；家长手机时钟可能不准，倒计时只信服务端。 */
function serverTimeOffset(meta: LotteryMeta): number {
  const server = Date.parse(meta.server_time);
  return Number.isNaN(server) ? 0 : server - Date.now();
}

/** 按服务端当前时间判定页面状态；未配置的时间窗口不做限制。 */
export function resolveStatus(
  meta: LotteryMeta,
  serverNow: number,
): LotteryStatus {
  const start = meta.start_at ? Date.parse(meta.start_at) : Number.NaN;
  const end = meta.end_at ? Date.parse(meta.end_at) : Number.NaN;
  if (!Number.isNaN(start) && serverNow < start) return 'not_started';
  if (!Number.isNaN(end) && serverNow >= end) return 'ended';
  return 'ongoing';
}

/** 每秒刷新的服务端时钟与状态。 */
export function useLotteryClock(meta: LotteryMeta | null): {
  status: LotteryStatus;
  serverNow: number;
} {
  const offset = useMemo(() => (meta ? serverTimeOffset(meta) : 0), [meta]);
  const [serverNow, setServerNow] = useState(() => Date.now() + offset);

  useEffect(() => {
    setServerNow(Date.now() + offset);
    const timer = window.setInterval(() => {
      setServerNow(Date.now() + offset);
    }, 1000);
    return () => window.clearInterval(timer);
  }, [offset]);

  const status = meta ? resolveStatus(meta, serverNow) : 'ongoing';
  return { status, serverNow };
}

const pad = (value: number) => String(value).padStart(2, '0');

/** 格式化赛事时间：11月25日 10:00。 */
function formatEventTime(iso: string): string {
  const ts = Date.parse(iso);
  if (Number.isNaN(ts)) return iso;
  const date = new Date(ts);
  return `${date.getMonth() + 1}月${date.getDate()}日 ${pad(
    date.getHours(),
  )}:${pad(date.getMinutes())}`;
}

/** 倒计时格式：超过一天显示「2天 03:04:05」，否则 03:04:05。 */
function formatCountdown(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const days = Math.floor(total / 86400);
  const hours = Math.floor((total % 86400) / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  const clock = `${pad(hours)}:${pad(minutes)}:${pad(seconds)}`;
  return days > 0 ? `${days}天 ${clock}` : clock;
}

/** 截止前两小时进入醒目状态（与短信提醒时点一致）。 */
const URGENT_MS = 2 * 60 * 60 * 1000;

/** 页面顶部抽签时间条：未开始显示开始时间，进行中显示截止倒计时，截止后提示自动分配。 */
const ScheduleBanner: React.FC<{
  meta: LotteryMeta;
  status: LotteryStatus;
  serverNow: number;
}> = ({ meta, status, serverNow }) => {
  if (status === 'ended') {
    return (
      <section className="lp-schedule is-ended" aria-live="polite">
        <p className="lp-schedule-label">抽签已结束</p>
        <p className="lp-schedule-text">
          未抽取的选手已由系统自动分配出场顺序，可继续查询结果。
        </p>
      </section>
    );
  }

  if (status === 'not_started') {
    const start = meta.start_at ? Date.parse(meta.start_at) : Number.NaN;
    return (
      <section className="lp-schedule is-waiting" aria-live="polite">
        <p className="lp-schedule-label">抽签尚未开始</p>
        {!Number.isNaN(start) && (
          <p className="lp-schedule-countdown">
            {formatCountdown(start - serverNow)}
          </p>
        )}
        {meta.start_at && (
          <p className="lp-schedule-text">
            {formatEventTime(meta.start_at)} 开始
          </p>
        )}
      </section>
    );
  }

  const endAt = meta.end_at;
  const end = endAt ? Date.parse(endAt) : Number.NaN;
  if (!endAt || Number.isNaN(end)) {
    return (
      <section className="lp-schedule is-ongoing" aria-live="polite">
        <p className="lp-schedule-label">抽签进行中</p>
      </section>
    );
  }

  const remaining = end - serverNow;
  const urgent = remaining <= URGENT_MS;
  return (
    <section
      className={`lp-schedule is-ongoing${urgent ? ' is-urgent' : ''}`}
      aria-live="polite"
    >
      <p className="lp-schedule-label">
        {urgent ? '即将截止，请尽快抽签' : '抽签进行中'}
      </p>
      <p className="lp-schedule-countdown">{formatCountdown(remaining)}</p>
      <p className="lp-schedule-text">
        {formatEventTime(endAt)} 截止
        {urgent ? '，逾期将由系统自动分配' : ''}
      </p>
    </section>
  );
};

export default ScheduleBanner;
