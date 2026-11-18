import React from 'react';
import type { LotteryGenre } from './service';

/** 顶部标识：黄色方形标记 + 优舞盟 + 赛事抽签。 */
export const BrandTopline: React.FC = () => (
  <div className="lp-topline">
    <span className="lp-brand">
      <span className="lp-brand-mark" aria-hidden="true">
        优
      </span>
      优舞盟
    </span>
    <span className="lp-topline-meta">赛事抽签</span>
  </div>
);

/** 正常页面标题：赛事名称 + 舞种列表。 */
export const PageHeader: React.FC<{
  eventName: string;
  genres: LotteryGenre[];
}> = ({ eventName, genres }) => (
  <header className="lp-header">
    <p className="lp-eyebrow">December · On Stage</p>
    <h1 className="lp-title">{eventName}</h1>
    <p className="lp-sub">{genres.map((item) => item.label).join(' / ')}</p>
  </header>
);

/** 链接无效时的标题：不展示抽签表单。 */
export const InvalidLinkHeader: React.FC = () => (
  <header className="lp-header">
    <p className="lp-eyebrow">Link Expired</p>
    <h1 className="lp-title">链接无效或已失效</h1>
    <p className="lp-sub">请联系赛事客服获取最新的抽签链接。</p>
  </header>
);
