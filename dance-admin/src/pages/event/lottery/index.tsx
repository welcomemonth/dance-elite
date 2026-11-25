import { PlusOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
import { App, Button, Space, Tag, Typography } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';
import { formatDateTime } from '@/services/youwumeng/format';
import {
  exportLotterySession,
  listLotterySessions,
} from '@/services/youwumeng/lottery';
import type { LotterySession } from '@/services/youwumeng/typings';
import DrawsModal from './draws-modal';
import ImportLotteryModal from './import-modal';
import SettingsLotteryModal from './settings-modal';

/** 时间窗口展示：未配置的一端显示为「不限」。 */
function windowText(record: LotterySession): string {
  if (!record.start_at && !record.end_at) {
    return '未限制';
  }
  const start = record.start_at ? formatDateTime(record.start_at) : '不限';
  const end = record.end_at ? formatDateTime(record.end_at) : '不限';
  return `${start} ~ ${end}`;
}

const EventLottery: React.FC = () => {
  const { message } = App.useApp();
  const access = useAccess();
  const actionRef = useRef<ActionType>(null);
  const [importOpen, setImportOpen] = useState(false);
  const [settingsToken, setSettingsToken] = useState<string | null>(null);
  const [drawsToken, setDrawsToken] = useState<string | null>(null);

  const handleExport = async (record: LotterySession) => {
    const filename = `lottery_${record.event_id ?? record.token}_${dayjs().format('YYYYMMDD')}.xlsx`;
    try {
      await exportLotterySession(record.token, filename);
      message.success('导出已开始，请查看浏览器下载');
    } catch (err) {
      message.error(
        err instanceof Error ? err.message : '导出失败，请稍后重试',
      );
    }
  };

  const columns: ProColumns<LotterySession>[] = [
    {
      title: '赛事',
      dataIndex: 'event_name',
      width: 260,
      render: (_, record) => (
        <div>
          <Typography.Text strong>{record.event_name}</Typography.Text>
          <div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {record.event_id ? `赛事 ID ${record.event_id}` : '未关联赛事'}
            </Typography.Text>
          </div>
        </div>
      ),
    },
    {
      title: '抽签链接',
      dataIndex: 'link',
      width: 320,
      render: (_, record) => (
        <div>
          <Typography.Text
            copyable={{ text: record.link }}
            ellipsis
            style={{ maxWidth: 280 }}
          >
            {record.link}
          </Typography.Text>
          <div>
            <Typography.Link
              href={record.link}
              target="_blank"
              rel="noreferrer"
              style={{ fontSize: 12 }}
            >
              打开链接
            </Typography.Link>
          </div>
        </div>
      ),
    },
    {
      title: '抽签进度',
      dataIndex: 'drawn_count',
      width: 170,
      render: (_, record) => (
        <div>
          <div>
            已抽 <strong>{record.drawn_count}</strong> / 共{' '}
            {record.player_count} 人
          </div>
          {record.pending_count > 0 ? (
            <Tag color="warning">未抽 {record.pending_count} 人</Tag>
          ) : (
            <Tag color="success">已全部抽完</Tag>
          )}
        </div>
      ),
    },
    {
      title: '抽签时间窗口',
      dataIndex: 'start_at',
      width: 230,
      render: (_, record) => (
        <span style={{ fontSize: 13 }}>{windowText(record)}</span>
      ),
    },
    {
      title: '更新时间',
      dataIndex: 'updated_at',
      width: 160,
      render: (_, record) => formatDateTime(record.updated_at),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 180,
      render: (_, record) => (
        <Space wrap size={4}>
          <Typography.Link onClick={() => setDrawsToken(record.token)}>
            抽签记录
          </Typography.Link>
          <Typography.Link onClick={() => setSettingsToken(record.token)}>
            设置
          </Typography.Link>
          {access.canAdmin && (
            <Typography.Link onClick={() => void handleExport(record)}>
              导出结果
            </Typography.Link>
          )}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer title={false}>
      <ProTable<LotterySession>
        headerTitle="赛事抽签"
        rowKey="token"
        actionRef={actionRef}
        columns={columns}
        search={false}
        pagination={false}
        request={async () => {
          const data = await listLotterySessions();
          return { data: data.list, total: data.list.length, success: true };
        }}
        toolBarRender={() =>
          access.canAdmin
            ? [
                <Button
                  key="import"
                  type="primary"
                  icon={<PlusOutlined />}
                  onClick={() => setImportOpen(true)}
                >
                  导入抽签 Excel
                </Button>,
              ]
            : []
        }
      />

      <ImportLotteryModal
        open={importOpen}
        onClose={() => setImportOpen(false)}
        onSuccess={(link) => {
          setImportOpen(false);
          actionRef.current?.reload();
          message.success(`抽签已生成：${link}`);
        }}
      />

      <SettingsLotteryModal
        token={settingsToken}
        open={Boolean(settingsToken)}
        onClose={() => setSettingsToken(null)}
        onSuccess={() => actionRef.current?.reload()}
      />

      <DrawsModal
        token={drawsToken}
        open={Boolean(drawsToken)}
        onClose={() => setDrawsToken(null)}
      />
    </PageContainer>
  );
};

export default EventLottery;
