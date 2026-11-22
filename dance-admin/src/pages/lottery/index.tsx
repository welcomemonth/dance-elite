import { useParams } from '@umijs/max';
import React, { useEffect, useState } from 'react';
import Field from './field';
import { BrandTopline, InvalidLinkHeader, PageHeader } from './header';
import ResultCard from './result-card';
import ScheduleBanner, { useLotteryClock } from './schedule';
import {
  drawLottery,
  fetchLotteryMeta,
  type LotteryMeta,
  type LotteryResult,
  queryLottery,
} from './service';
import './style.css';

type Tab = 'draw' | 'query';
type LoadState = 'loading' | 'error' | 'done';

const LotteryPage: React.FC = () => {
  const { token } = useParams<{ token: string }>();

  const [meta, setMeta] = useState<LotteryMeta | null>(null);
  const [loadState, setLoadState] = useState<LoadState>('loading');
  const [tab, setTab] = useState<Tab>('draw');
  const [name, setName] = useState('');
  const [idCard, setIdCard] = useState('');
  const [genre, setGenre] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState<LotteryResult | null>(null);
  const [fresh, setFresh] = useState(false);
  const clock = useLotteryClock(meta);

  useEffect(() => {
    if (!token) {
      setLoadState('error');
      return;
    }
    let cancelled = false;
    fetchLotteryMeta(token)
      .then((data) => {
        if (cancelled) return;
        setMeta(data);
        setGenre(data.genres[0]?.key ?? '');
        setLoadState('done');
        document.title = `抽签 | ${data.event_name}`;
      })
      .catch(() => {
        if (!cancelled) setLoadState('error');
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  const clearResult = () => {
    setResult(null);
    setFresh(false);
    setError('');
  };

  const switchTab = (next: Tab) => {
    if (next === tab) return;
    setTab(next);
    clearResult();
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!token || busy) return;

    if (tab === 'draw' && clock.status !== 'ongoing') {
      setError(
        clock.status === 'not_started'
          ? '抽签尚未开始，请到开始后再试。'
          : '抽签已结束，未抽签的选手已由系统自动分配，请使用「查询」。',
      );
      return;
    }

    const trimmedName = name.trim();
    const trimmedID = idCard.trim();
    if (tab === 'draw' && !trimmedName) {
      setError('请输入姓名。');
      return;
    }
    if (!trimmedID) {
      setError('请输入身份证号。');
      return;
    }
    if (trimmedID.length !== 18) {
      setError('请输入 18 位身份证号。');
      return;
    }
    if (!genre) {
      setError('请选择舞种。');
      return;
    }

    setBusy(true);
    setError('');
    try {
      const data =
        tab === 'draw'
          ? await drawLottery(token, {
              name: trimmedName,
              id_card: trimmedID,
              genre,
            })
          : await queryLottery(token, { id_card: trimmedID, genre });
      setFresh(data.just_drawn);
      setResult(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : '操作失败，请稍后重试。');
    } finally {
      setBusy(false);
    }
  };

  if (loadState === 'loading') {
    return (
      <div className="lottery-page">
        <p className="lp-state-text">加载中…</p>
      </div>
    );
  }

  if (loadState === 'error' || !meta) {
    return (
      <div className="lottery-page">
        <BrandTopline />
        <InvalidLinkHeader />
      </div>
    );
  }

  const isDraw = tab === 'draw';
  const canDraw = clock.status === 'ongoing';

  return (
    <div className="lottery-page">
      <BrandTopline />
      <PageHeader eventName={meta.event_name} genres={meta.genres} />
      <ScheduleBanner
        meta={meta}
        status={clock.status}
        serverNow={clock.serverNow}
      />

      <div className="lp-tabs" role="tablist" aria-label="抽签或查询">
        <button
          type="button"
          role="tab"
          aria-selected={isDraw}
          className={`lp-tab${isDraw ? ' is-active' : ''}`}
          onClick={() => switchTab('draw')}
        >
          抽签
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={!isDraw}
          className={`lp-tab${!isDraw ? ' is-active' : ''}`}
          onClick={() => switchTab('query')}
        >
          查询
        </button>
      </div>

      {result ? (
        <>
          <ResultCard
            result={result}
            fresh={fresh}
            onGoDraw={() => {
              setResult(null);
              setFresh(false);
              setTab('draw');
            }}
          />
          <button type="button" className="lp-back" onClick={clearResult}>
            ← 返回重新输入
          </button>
        </>
      ) : isDraw && !canDraw ? (
        <div className="lp-locked">
          <p className="lp-empty-title">
            {clock.status === 'not_started' ? '尚未开始' : '抽签已结束'}
          </p>
          <p className="lp-empty-text">
            {clock.status === 'not_started'
              ? '抽签开始后才能抽取你的出场顺序，现在可以先查询已有结果。'
              : '截止后未抽签的选手已由系统自动分配出场顺序，请到「查询」查看结果。'}
          </p>
          <button
            type="button"
            className="lp-text-btn"
            onClick={() => switchTab('query')}
          >
            去查询 →
          </button>
        </div>
      ) : (
        <form className="lp-form" onSubmit={handleSubmit} noValidate>
          {isDraw && (
            <Field id="lottery-name" label="姓名">
              <input
                id="lottery-name"
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="报名时填写的姓名"
                autoComplete="off"
                spellCheck={false}
                maxLength={30}
              />
            </Field>
          )}

          <Field id="lottery-id" label="身份证号">
            <input
              id="lottery-id"
              type="text"
              value={idCard}
              onChange={(e) => setIdCard(e.target.value)}
              placeholder="18 位身份证号"
              autoComplete="off"
              spellCheck={false}
              inputMode="text"
              maxLength={18}
            />
          </Field>

          <Field id="lottery-genre" label="舞种">
            <span className="lp-select-wrap">
              <select
                id="lottery-genre"
                value={genre}
                onChange={(e) => setGenre(e.target.value)}
              >
                {meta.genres.map((item) => (
                  <option key={item.key} value={item.key}>
                    {item.label}
                  </option>
                ))}
              </select>
              <span className="lp-select-arrow" aria-hidden="true">
                ▾
              </span>
            </span>
          </Field>

          <button type="submit" className="lp-submit" disabled={busy}>
            {busy ? '处理中…' : isDraw ? '抽取出场顺序' : '查询我的结果'}
            {!busy && <span aria-hidden="true">→</span>}
          </button>

          {error && (
            <p className="lp-error" role="alert">
              {error}
            </p>
          )}

          <p className="lp-hint">
            {isDraw
              ? '姓名、证件号、舞种三项需与报名时一致；每人只能抽一次。'
              : '输入报名时的证件号和舞种即可查询，无需姓名。'}
          </p>
        </form>
      )}

      <footer className="lp-footer">
        <span className="lp-footer-dot" aria-hidden="true" />
        本链接仅用于本次赛事抽签，请勿转发。
      </footer>
    </div>
  );
};

export default LotteryPage;
