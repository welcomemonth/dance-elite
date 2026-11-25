import { Drawer, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import React, { useEffect, useState } from 'react';
import { formatDateTime } from '@/services/youwumeng/format';
import { getLotterySession } from '@/services/youwumeng/lottery';
import type {
  LotteryDraw,
  LotterySessionDetail,
} from '@/services/youwumeng/typings';

interface DrawsModalProps {
  token: string | null;
  open: boolean;
  onClose: () => void;
}

/** 抽签微信标识展示：保留前 6 位与后 4 位，完整值可复制。 */
function maskOpenID(openid: string): string {
  if (openid.length <= 10) return openid;
  return `${openid.slice(0, 6)}…${openid.slice(-4)}`;
}

const columns: ColumnsType<LotteryDraw> = [
  { title: '选手', dataIndex: 'player_name', width: 100 },
  { title: '舞种', dataIndex: 'genre_label', width: 110 },
  { title: '年龄组', dataIndex: 'age_group', width: 80 },
  {
    title: '槽位',
    width: 80,
    render: (_, record) =>
      record.slot ? `${record.slot.letter}${record.slot.no}` : '—',
  },
  {
    title: '抽签时间',
    dataIndex: 'drawn_at',
    width: 160,
    render: (value?: string) => (value ? formatDateTime(value) : '—'),
  },
  {
    title: '方式',
    dataIndex: 'auto_drawn',
    width: 100,
    render: (auto: boolean) =>
      auto ? <Tag>系统自动</Tag> : <Tag color="blue">本人抽取</Tag>,
  },
  {
    title: '抽签微信标识',
    dataIndex: 'openid',
    width: 160,
    render: (openid?: string) =>
      openid ? (
        <Typography.Text copyable={{ text: openid }} style={{ fontSize: 12 }}>
          {maskOpenID(openid)}
        </Typography.Text>
      ) : (
        '—'
      ),
  },
  {
    title: '同微信抽签',
    dataIndex: 'same_wechat_count',
    width: 100,
    render: (count?: number) =>
      count && count > 1 ? <Tag color="warning">{count} 人</Tag> : '—',
  },
  {
    title: '匹配小程序账号',
    width: 190,
    render: (_, record) => {
      if (record.user) {
        const name = record.user.nickname || `用户 #${record.user.id}`;
        return record.user.phone ? `${name} · ${record.user.phone}` : name;
      }
      return record.auto_drawn ? '—' : '未匹配';
    },
  },
];

const DrawsModal: React.FC<DrawsModalProps> = ({ token, open, onClose }) => {
  const [detail, setDetail] = useState<LotterySessionDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!open || !token) return;
    let cancelled = false;
    setLoading(true);
    setError('');
    getLotterySession(token)
      .then((data) => {
        if (!cancelled) setDetail(data);
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(
            err instanceof Error ? err.message : '读取抽签记录失败，请稍后重试',
          );
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, token]);

  return (
    <Drawer
      title={
        detail
          ? `抽签记录（已抽 ${detail.drawn_count} / 共 ${detail.player_count} 人）`
          : '抽签记录'
      }
      open={open}
      width={960}
      onClose={onClose}
    >
      {error ? <Typography.Text type="danger">{error}</Typography.Text> : null}
      <Table<LotteryDraw>
        rowKey={(record) => `${record.player_name}-${record.genre}`}
        columns={columns}
        dataSource={detail?.draws ?? []}
        loading={loading}
        size="small"
        pagination={{ pageSize: 20, showSizeChanger: false }}
        locale={{ emptyText: '暂无抽签记录' }}
      />
      <Typography.Paragraph
        type="secondary"
        style={{ marginTop: 16, fontSize: 12 }}
      >
        抽签微信标识为公众号网页授权获取的 openid；「匹配小程序账号」按 unionid
        关联报名时登录的微信账号，未匹配表示不是报名账号本人操作（例如其他家长代抽）。
        点击标识右侧图标可复制完整值。
      </Typography.Paragraph>
    </Drawer>
  );
};

export default DrawsModal;
