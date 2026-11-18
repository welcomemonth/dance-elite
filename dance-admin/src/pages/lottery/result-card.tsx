import React from 'react';
import type { LotteryResult } from './service';

/** 抽签结果卡片：展示「字母组 + 组内序号」，首次抽中时播放翻牌与盖章动画。 */
const ResultCard: React.FC<{
  result: LotteryResult;
  fresh: boolean;
  onGoDraw: () => void;
}> = ({ result, fresh, onGoDraw }) => {
  const slot = result.slot;
  return (
    <section
      className={`lp-card${fresh ? ' is-fresh' : ''}`}
      aria-live="polite"
    >
      <div className="lp-card-strip">
        <span>出场顺序</span>
        <span className="lp-card-strip-en">Stage Call</span>
      </div>

      {slot ? (
        <div className="lp-card-body">
          <p className="lp-card-scope">
            {result.age_group} · {result.genre_label}
          </p>
          <div className="lp-slot">
            <span className="lp-slot-letter">{slot.letter}</span>
            <span className="lp-slot-no">{slot.no}</span>
          </div>
          <p className="lp-card-order">
            {slot.letter} 组 · 第 {slot.no} 位出场
          </p>

          <dl className="lp-card-facts">
            <div>
              <dt>选手</dt>
              <dd>{result.name}</dd>
            </div>
            {result.program && (
              <div>
                <dt>剧目</dt>
                <dd>{result.program}</dd>
              </div>
            )}
            {result.institution && (
              <div>
                <dt>机构</dt>
                <dd>{result.institution}</dd>
              </div>
            )}
          </dl>

          <span className="lp-stamp" aria-hidden="true">
            已抽签
          </span>
        </div>
      ) : (
        <div className="lp-card-body lp-card-empty">
          <p className="lp-empty-title">尚未抽签</p>
          <p className="lp-empty-text">
            {result.name} 还没有抽取出场顺序，请先完成抽签。
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
