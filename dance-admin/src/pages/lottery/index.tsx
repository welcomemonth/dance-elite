import { useParams } from '@umijs/max';
import React, { useCallback, useEffect, useRef, useState } from 'react';
import Field from './field';
import { BrandTopline, InvalidLinkHeader, PageHeader } from './header';
import { renderLotteryPoster } from './poster';
import ResultCard from './result-card';
import ScheduleBanner, { useLotteryClock } from './schedule';
import {
  drawLottery,
  fetchLotteryMeta,
  type LotteryDrawPayload,
  type LotteryDrawResult,
  type LotteryEntry,
  type LotteryGenre,
  type LotteryMeta,
  lookupLottery,
  queryLottery,
} from './service';
import { formatDrawnAt } from './time';
import './style.css';

type Tab = 'draw' | 'query';
type LoadState = 'loading' | 'error' | 'done';

/** localStorage 记住上次输入的 token 维度键（换赛事互不影响）。 */
function storageKey(token: string): string {
  return `dance-elite:lottery:${token}`;
}

interface Remembered {
  name: string;
  id_card: string;
}

function loadRemembered(token: string): Remembered | null {
  try {
    const raw = window.localStorage.getItem(storageKey(token));
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Remembered;
    return parsed?.id_card ? parsed : null;
  } catch {
    return null;
  }
}

function saveRemembered(token: string, value: Remembered): void {
  try {
    window.localStorage.setItem(storageKey(token), JSON.stringify(value));
  } catch {
    // 隐私模式下 localStorage 可能不可用，忽略即可。
  }
}

function clearRemembered(token: string): void {
  try {
    window.localStorage.removeItem(storageKey(token));
  } catch {
    // 同上。
  }
}

/** 判断服务端错误是否属于「未查询到/信息不匹配」，用于展示醒目的排查面板。 */
function isNotFoundMessage(message: string): boolean {
  return (
    message.includes('未查询到') ||
    message.includes('信息不匹配') ||
    message.includes('未找到')
  );
}

/** 从记录中取出最近一次抽签时间。 */
function latestDrawnAt(entries: LotteryEntry[]): string {
  let latest = '';
  for (const entry of entries) {
    if (entry.drawn && entry.drawn_at && entry.drawn_at > latest) {
      latest = entry.drawn_at;
    }
  }
  return latest;
}

const LotteryPage: React.FC = () => {
  const { token } = useParams<{ token: string }>();

  const [meta, setMeta] = useState<LotteryMeta | null>(null);
  const [loadState, setLoadState] = useState<LoadState>('loading');
  const [tab, setTab] = useState<Tab>('draw');
  const [name, setName] = useState('');
  const [idCard, setIdCard] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notFound, setNotFound] = useState('');
  const [notice, setNotice] = useState('');
  const [entries, setEntries] = useState<LotteryEntry[] | null>(null);
  const [fresh, setFresh] = useState(false);
  const [choice, setChoice] = useState<{
    choices: LotteryGenre[];
    payload: LotteryDrawPayload;
  } | null>(null);
  const [poster, setPoster] = useState('');
  const restoredRef = useRef(false);
  const clock = useLotteryClock(meta);

  /** 统一收敛一次查询/抽签结果。 */
  const applyEntries = useCallback(
    (result: LotteryEntry[], options: { fresh?: boolean; note?: string }) => {
      setEntries(result);
      setNotFound('');
      setError('');
      setFresh(Boolean(options.fresh));
      const drawnAt = latestDrawnAt(result);
      if (options.note) {
        setNotice(options.note);
      } else if (drawnAt) {
        setNotice(`已于 ${formatDrawnAt(drawnAt)} 完成抽签`);
      } else {
        setNotice('已查询到报名信息，尚未抽签。');
      }
    },
    [],
  );

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

  // 再次打开页面时自动带出上次输入的证件号并展示结果（需求：提示上次抽签时间）。
  useEffect(() => {
    if (!token || !meta || restoredRef.current) return;
    restoredRef.current = true;
    const remembered = loadRemembered(token);
    if (!remembered) return;
    setName(remembered.name ?? '');
    setIdCard(remembered.id_card);
    lookupLottery(token, { id_card: remembered.id_card })
      .then((data) => {
        if (data.entries.length === 0) return;
        setTab('draw');
        applyEntries(data.entries, {});
      })
      .catch(() => {
        // 上次的证件号可能已不在本场名单（例如重新导入），静默忽略。
      });
  }, [token, meta, applyEntries]);

  const resetView = () => {
    setEntries(null);
    setFresh(false);
    setError('');
    setNotFound('');
    setNotice('');
    setChoice(null);
    setPoster('');
  };

  const switchTab = (next: Tab) => {
    if (next === tab) return;
    setTab(next);
    resetView();
  };

  const goDraw = () => {
    setTab('draw');
    resetView();
  };

  const validate = (needName: boolean): boolean => {
    const trimmedID = idCard.trim();
    if (needName && !name.trim()) {
      setError('请输入姓名。');
      return false;
    }
    if (!trimmedID) {
      setError('请输入身份证号。');
      return false;
    }
    if (trimmedID.length !== 18) {
      setError('请输入 18 位身份证号。');
      return false;
    }
    return true;
  };

  const submitDraw = async (payload: LotteryDrawPayload) => {
    if (!token || busy) return;
    setBusy(true);
    setError('');
    setNotFound('');
    try {
      const data = await drawLottery(token, payload);
      saveRemembered(token, { name: payload.name, id_card: payload.id_card });
      handleDrawResult(data, payload);
    } catch (err) {
      const message = err instanceof Error ? err.message : '操作失败，请稍后重试。';
      if (isNotFoundMessage(message)) {
        setNotFound(message);
      } else {
        setError(message);
      }
    } finally {
      setBusy(false);
    }
  };

  const showPoster = (nextEntries: LotteryEntry[]) => {
    if (!meta || !nextEntries.some((entry) => entry.slot)) return;
    const url = renderLotteryPoster(meta.event_name, nextEntries);
    if (url) {
      setPoster(url);
    } else {
      setError('生成图片失败，请使用手机截图保存。');
    }
  };

  const handleDrawResult = (data: LotteryDrawResult, payload: LotteryDrawPayload) => {
    if (data.status === 'choose_genre') {
      setChoice({ choices: data.choices ?? [], payload });
      return;
    }
    setChoice(null);
    if (data.status === 'already_drawn') {
      applyEntries(data.entries, {});
      showPoster(data.entries);
      return;
    }
    const pending = data.entries.filter((entry) => !entry.drawn).length;
    applyEntries(data.entries, {
      fresh: true,
      note:
        pending > 0
          ? `本次抽签已完成；还有 ${pending} 个舞种未抽签，请继续完成。`
          : '抽签已完成，结果图片已生成，可长按保存。',
    });
    showPoster(data.entries);
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
    if (!validate(tab === 'draw')) return;

    const trimmedName = name.trim();
    const trimmedID = idCard.trim();

    if (tab === 'query') {
      setBusy(true);
      setError('');
      setNotFound('');
      try {
        const data = await queryLottery(token, { id_card: trimmedID });
        saveRemembered(token, { name: trimmedName, id_card: trimmedID });
        applyEntries(data.entries, {});
      } catch (err) {
        const message = err instanceof Error ? err.message : '查询失败，请稍后重试。';
        if (isNotFoundMessage(message)) {
          setNotFound(message);
        } else {
          setError(message);
        }
      } finally {
        setBusy(false);
      }
      return;
    }

    await submitDraw({ name: trimmedName, id_card: trimmedID });
  };

  const openPoster = () => {
    if (entries) showPoster(entries);
  };

  const closePoster = () => setPoster('');

  const forgetMe = () => {
    if (token) clearRemembered(token);
    setName('');
    setIdCard('');
    resetView();
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
  const drawnEntries = entries?.filter((entry) => entry.drawn) ?? [];
  const pendingEntries = entries?.filter((entry) => !entry.drawn) ?? [];

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

      {notFound ? (
        <section className="lp-notfound" role="alert">
          <p className="lp-notfound-code">NO RECORD · 未查到记录</p>
          <h2 className="lp-notfound-title">没有找到报名信息</h2>
          <p className="lp-notfound-detail">{notFound}</p>
          <div className="lp-notfound-block">
            <p className="lp-notfound-sub">请先自查：</p>
            <ul className="lp-notfound-list">
              <li>身份证号是否为 18 位，末位 X 是否大写、前后无空格；</li>
              <li>姓名是否与报名时完全一致（不要填家长姓名）；</li>
              <li>报名是否已完成支付（未支付不会进入抽签名单）。</li>
            </ul>
          </div>
          <p className="lp-notfound-help">
            确认已报名仍查不到？请把「选手姓名 + 身份证号」发给赛事客服核对，
            无需重复提交，也请勿相信非官方渠道。
          </p>
          <button type="button" className="lp-notfound-back" onClick={resetView}>
            返回重新输入
          </button>
        </section>
      ) : entries ? (
        <>
          {notice && (
            <section className="lp-notice" aria-live="polite">
              <span className="lp-notice-dot" aria-hidden="true" />
              {notice}
            </section>
          )}

          <div className="lp-results">
            {entries.map((entry) => (
              <ResultCard
                key={entry.genre}
                entry={entry}
                fresh={fresh && entry.drawn}
                onGoDraw={goDraw}
                onOpenPoster={
                  isDraw ? undefined : () => showPoster([entry])
                }
              />
            ))}
          </div>

          {pendingEntries.length > 0 && drawnEntries.length > 0 && (
            <p className="lp-pending-hint">
              还有 {pendingEntries.length} 个舞种未抽签，请点击上方卡片中的「去抽签」完成。
            </p>
          )}

          {!isDraw && drawnEntries.length > 0 && (
            <p className="lp-pending-hint">
              点击上方已抽签的卡片，可查看并保存该舞种的结果图片。
            </p>
          )}

          <div className="lp-actions">
            {isDraw && drawnEntries.length > 0 && (
              <button type="button" className="lp-submit" onClick={openPoster}>
                生成结果图片
                <span aria-hidden="true">↓</span>
              </button>
            )}
            <button type="button" className="lp-back" onClick={resetView}>
              ← 返回重新输入
            </button>
            <button type="button" className="lp-back" onClick={forgetMe}>
              不是我，清除本机记住的信息
            </button>
          </div>
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
                placeholder="报名时填写的选手姓名"
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
              ? '输入姓名和证件号即可抽签，无需手动选择舞种；同一选手报名多个舞种时会提示选择。'
              : '只需输入报名时的身份证号，即可查看该选手全部已抽签信息。'}
          </p>
        </form>
      )}

      {choice && (
        <div className="lp-modal-mask" role="dialog" aria-modal="true">
          <div className="lp-modal">
            <p className="lp-eyebrow">Multiple Entries</p>
            <h2 className="lp-modal-title">请选择要抽签的舞种</h2>
            <p className="lp-modal-text">
              检测到该选手报名了 {choice.choices.length} 个舞种，请选择本次要抽取的舞种。
            </p>
            <div className="lp-modal-choices">
              {choice.choices.map((item) => (
                <button
                  key={item.key}
                  type="button"
                  className="lp-choice"
                  disabled={busy}
                  onClick={() => {
                    const payload = { ...choice.payload, genre: item.key };
                    setChoice(null);
                    void submitDraw(payload);
                  }}
                >
                  {item.label}
                  <span aria-hidden="true">→</span>
                </button>
              ))}
            </div>
            <button
              type="button"
              className="lp-modal-cancel"
              onClick={() => setChoice(null)}
            >
              取消
            </button>
          </div>
        </div>
      )}

      {poster && (
        <div className="lp-modal-mask lp-poster-mask" role="dialog" aria-modal="true">
          <div className="lp-poster">
            <p className="lp-poster-hint">长按图片保存到手机相册</p>
            <img src={poster} alt="抽签结果，长按保存" />
            <button type="button" className="lp-modal-cancel" onClick={closePoster}>
              关闭
            </button>
          </div>
        </div>
      )}

      <footer className="lp-footer">
        <span className="lp-footer-dot" aria-hidden="true" />
        本链接仅用于本次赛事抽签，请勿转发。
      </footer>
    </div>
  );
};

export default LotteryPage;
