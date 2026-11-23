import type { LotteryEntry } from './service';
import { formatDrawnAt } from './time';

/** 海报逻辑宽度（CSS px）；实际画布按 2 倍分辨率导出，长按保存更清晰。 */
const POSTER_WIDTH = 750;
const SCALE = 2;

const INK = '#121217';
const MUTED = '#6c6c89';
const YELLOW = '#ffc233';
const PURPLE = '#5423e7';
const PAPER = '#ffffff';

const displayFont = '"Times New Roman", Times, "Songti SC", STSong, SimSun, serif';
const uiFont =
  '-apple-system, BlinkMacSystemFont, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif';

/** 每条记录所占高度。 */
const ENTRY_HEIGHT = 300;
const HEADER_HEIGHT = 230;
const FOOTER_HEIGHT = 150;

/**
 * 把抽签结果绘制成一张可长按保存的 PNG（data URL）。
 *
 * 使用原生 Canvas 而不是 html2canvas：H5 页面体积小、无需额外依赖，
 * 且 Canvas 在微信内置浏览器里长按保存的兼容性最好。
 */
export function renderLotteryPoster(
  eventName: string,
  entries: LotteryEntry[],
): string {
  const drawn = entries.filter((entry) => entry.slot);
  const height = HEADER_HEIGHT + Math.max(drawn.length, 1) * ENTRY_HEIGHT + FOOTER_HEIGHT;

  const canvas = document.createElement('canvas');
  canvas.width = POSTER_WIDTH * SCALE;
  canvas.height = height * SCALE;
  const ctx = canvas.getContext('2d');
  if (!ctx) {
    return '';
  }
  ctx.scale(SCALE, SCALE);
  ctx.textBaseline = 'alphabetic';

  // 纸张与边框
  ctx.fillStyle = PAPER;
  ctx.fillRect(0, 0, POSTER_WIDTH, height);
  ctx.strokeStyle = INK;
  ctx.lineWidth = 4;
  ctx.strokeRect(2, 2, POSTER_WIDTH - 4, height - 4);

  // 顶部黄色条
  ctx.fillStyle = YELLOW;
  ctx.fillRect(2, 2, POSTER_WIDTH - 4, 86);
  ctx.fillStyle = INK;
  ctx.font = `700 30px ${uiFont}`;
  ctx.fillText('优舞盟', 40, 56);
  ctx.font = `600 20px ${uiFont}`;
  ctx.textAlign = 'right';
  ctx.fillText('赛事抽签 · LOTTERY', POSTER_WIDTH - 40, 56);
  ctx.textAlign = 'left';

  // 赛事名称
  ctx.fillStyle = INK;
  ctx.font = `700 40px ${displayFont}`;
  fillWrapped(ctx, eventName, 40, 156, POSTER_WIDTH - 80, 48, 2);

  ctx.fillStyle = MUTED;
  ctx.font = `600 18px ${uiFont}`;
  ctx.fillText('出场顺序 · STAGE CALL', 40, HEADER_HEIGHT - 22);

  ctx.strokeStyle = INK;
  ctx.lineWidth = 2;
  ctx.beginPath();
  ctx.moveTo(40, HEADER_HEIGHT);
  ctx.lineTo(POSTER_WIDTH - 40, HEADER_HEIGHT);
  ctx.stroke();

  drawn.forEach((entry, index) => {
    drawEntry(ctx, entry, HEADER_HEIGHT + index * ENTRY_HEIGHT);
  });

  // 页脚
  const footerTop = height - FOOTER_HEIGHT;
  ctx.strokeStyle = INK;
  ctx.lineWidth = 2;
  ctx.beginPath();
  ctx.moveTo(40, footerTop);
  ctx.lineTo(POSTER_WIDTH - 40, footerTop);
  ctx.stroke();

  ctx.fillStyle = MUTED;
  ctx.font = `500 18px ${uiFont}`;
  ctx.fillText('请按抽签场次提前候场，迟到可能顺延出场。', 40, footerTop + 46);
  ctx.fillText('本图片由优舞盟赛事系统生成，仅供本人留存。', 40, footerTop + 80);

  return canvas.toDataURL('image/png');
}

/** 绘制一条记录。 */
function drawEntry(
  ctx: CanvasRenderingContext2D,
  entry: LotteryEntry,
  top: number,
): void {
  if (!entry.slot) {
    return;
  }
  const left = 40;
  const width = POSTER_WIDTH - 80;

  // 姓名与舞种
  ctx.fillStyle = MUTED;
  ctx.font = `600 20px ${uiFont}`;
  ctx.fillText(`${entry.age_group} · ${entry.genre_label}`, left, top + 46);

  ctx.fillStyle = INK;
  ctx.font = `700 30px ${uiFont}`;
  ctx.fillText(entry.name, left, top + 84);

  if (entry.program) {
    ctx.fillStyle = MUTED;
    ctx.font = `500 18px ${uiFont}`;
    ctx.fillText(truncate(entry.program, 26), left, top + 114);
  }

  // 大号槽位
  ctx.fillStyle = INK;
  ctx.font = `700 110px ${displayFont}`;
  const letter = entry.slot.letter;
  ctx.fillText(letter, left, top + 220);
  const letterWidth = ctx.measureText(letter).width;
  ctx.font = `700 150px ${displayFont}`;
  ctx.fillText(String(entry.slot.no), left + letterWidth + 14, top + 236);

  // 出场说明
  ctx.fillStyle = INK;
  ctx.font = `600 22px ${uiFont}`;
  ctx.fillText(`${letter} 组 · 第 ${entry.slot.no} 位出场`, left, top + 274);

  // 抽签时间 / 自动分配
  ctx.textAlign = 'right';
  ctx.fillStyle = MUTED;
  ctx.font = `500 18px ${uiFont}`;
  const timeText = entry.drawn_at ? `抽签时间 ${formatDrawnAt(entry.drawn_at)}` : '';
  if (timeText) {
    ctx.fillText(timeText, left + width, top + 46);
  }
  if (entry.auto_drawn) {
    ctx.fillStyle = PURPLE;
    ctx.font = `600 18px ${uiFont}`;
    ctx.fillText('截止后系统自动分配', left + width, top + 74);
  }

  // 印章
  ctx.save();
  ctx.translate(left + width - 70, top + 190);
  ctx.rotate((-8 * Math.PI) / 180);
  ctx.strokeStyle = PURPLE;
  ctx.lineWidth = 4;
  ctx.strokeRect(-56, -30, 112, 60);
  ctx.fillStyle = PURPLE;
  ctx.font = `700 24px ${uiFont}`;
  ctx.textAlign = 'center';
  ctx.fillText('已抽签', 0, 8);
  ctx.restore();
  ctx.textAlign = 'left';

  // 分隔线
  ctx.strokeStyle = '#e7e6e1';
  ctx.lineWidth = 1;
  ctx.beginPath();
  ctx.moveTo(left, top + ENTRY_HEIGHT - 16);
  ctx.lineTo(left + width, top + ENTRY_HEIGHT - 16);
  ctx.stroke();
}

/** 自动换行绘制文本，最多 maxLines 行，超出以省略号结尾。 */
function fillWrapped(
  ctx: CanvasRenderingContext2D,
  text: string,
  x: number,
  y: number,
  maxWidth: number,
  lineHeight: number,
  maxLines: number,
): void {
  const chars = Array.from(text);
  let line = '';
  let lineIndex = 0;
  for (let i = 0; i < chars.length; i += 1) {
    const next = line + chars[i];
    if (ctx.measureText(next).width > maxWidth && line) {
      if (lineIndex === maxLines - 1) {
        ctx.fillText(`${line.slice(0, -1)}…`, x, y + lineIndex * lineHeight);
        return;
      }
      ctx.fillText(line, x, y + lineIndex * lineHeight);
      lineIndex += 1;
      line = chars[i];
    } else {
      line = next;
    }
  }
  ctx.fillText(line, x, y + lineIndex * lineHeight);
}

function truncate(text: string, max: number): string {
  const chars = Array.from(text);
  return chars.length > max ? `${chars.slice(0, max).join('')}…` : text;
}
