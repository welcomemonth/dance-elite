import React from 'react';
import type { LotteryEntry } from './service';
import { formatDrawnAt } from './time';

/** 抽签结果卡片：一条记录（一个舞种）一张卡，展示「字母组 + 组内序号」与抽签时间。 */
const ResultCard: React.FC<{
  entry: LotteryEntry;
  fresh: boolean;
  onGoDraw: () => void;
  /** 传入后已抽签的卡片可点击，生成该条记录的图片 */
  onOpenPoster?: () => void;
}> = ({ entry, fresh, onGoDraw, onOpenPoster }) => {
  const slot = entry.slot;
  const clickable = Boolean(onOpenPoster && slot);
  return (
    <section
      className={`lp-card${fresh ? ' is-fresh' : ''}${clickable ? ' is-clickable' : ''}`}
      aria-live="polite"
      role={clickable ? 'button' : undefined}
      tabIndex={clickable ? 0 : undefined}
      onClick={clickable ? onOpenPoster : undefined}
      onKeyDown={
        clickable
          ? (event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault();
                onOpenPoster?.();
              }
            }
          : undefined
      }
    >
      <div className="lp-card-strip">
        <span>{entry.drawn ? '出场顺序' : '等待抽签'}</span>
        <span className="lp-card-strip-en">
          {entry.genre_label} · {entry.age_group}
        </span>
      </div>

      {slot ? (
        <div className="lp-card-body">
          <div className="lp-slot">
            <span className="lp-slot-letter">{slot.letter}</span>
            <span className="lp-slot-no">{slot.no}</span>
          </div>
          <p className="lp-card-order">
            {slot.letter} 组 · 第 {slot.no} 位出场
          </p>
          {entry.auto_drawn && (
            <p className="lp-card-note">抽签截止后由系统自动分配</p>
          )}

          <dl className="lp-card-facts">
            <div>
              <dt>选手</dt>
              <dd>{entry.name}</dd>
            </div>
            <div>
              <dt>舞种</dt>
              <dd>
                {entry.genre_label} · {entry.age_group}
              </dd>
            </div>
            {entry.program && (
              <div>
                <dt>剧目</dt>
                <dd>{entry.program}</dd>
              </div>
            )}
            {entry.institution && (
              <div>
                <dt>机构</dt>
                <dd>{entry.institution}</dd>
              </div>
            )}
            {entry.drawn_at && (
              <div>
                <dt>抽签</dt>
                <dd>{formatDrawnAt(entry.drawn_at)}</dd>
              </div>
            )}
          </dl>

          {onOpenPoster && (
            <p className="lp-card-tap-hint">点击卡片查看结果图片 →</p>
          )}

          <span className="lp-stamp" aria-hidden="true">
            已抽签
          </span>
        </div>
      ) : (
        <div className="lp-card-body lp-card-empty">
          <p className="lp-empty-title">尚未抽签</p>
          <p className="lp-empty-text">
            {entry.name} 的{entry.genre_label}还没有抽取出场顺序，请先完成抽签。
          </p>
          <button type="button" className="lp-text-btn" onClick={onGoDraw}>
            去抽签 →
          </button>
        </div>
      )}
    </section>
  );
};

export default ResultCard;
