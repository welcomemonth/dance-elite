import type { ActionType, ProColumns, ProFormInstance } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { history, useAccess, useSearchParams } from '@umijs/max';
import { App, Button, Modal, Space, Tag, Typography } from 'antd';
import React, { useRef, useState } from 'react';
import {
  DANCE_TYPE_LABELS,
  MISSING_ITEM_LABELS,
} from '@/services/youwumeng/labels';
import { formatDateTime } from '@/services/youwumeng/format';
import { listEvents } from '@/services/youwumeng/events';
import {
  exportMissingMaterials,
  listMissingMaterials,
  markMaterialContacted,
} from '@/services/youwumeng/materials';
import type { MissingItem, MissingMaterialEntry } from '@/services/youwumeng/typings';

/** 资料催收：未传齐名单（设计文档 7.1/7.2） */
const MaterialsList: React.FC = () => {
  const { message } = App.useApp();
  const access = useAccess();
  const actionRef = useRef<ActionType>(null);
  const formRef = useRef<ProFormInstance>(undefined);
  const [searchParams] = useSearchParams();
  const urlEventId = searchParams.get('event_id')
    ? Number(searchParams.get('event_id'))
    : undefined;
  const [contactTarget, setContactTarget] = useState<MissingMaterialEntry | null>(null);
  const [contactNote, setContactNote] = useState('');

  const columns: ProColumns<MissingMaterialEntry>[] = [
    {
      title: '赛事',
      dataIndex: 'event_name',
      width: 220,
      render: (_, r) => <span>{r.event_name}</span>,
    },
    {
      title: '赛事',
      dataIndex: 'event_id',
      hideInTable: true,
      valueType: 'select',
      request: async () => {
        const data = await listEvents({ page: 1, page_size: 100 });
        return data.list.map((e) => ({ label: e.name, value: e.id }));
      },
    },
    {
      title: '缺失项',
      dataIndex: 'missing_item',
      hideInTable: true,
      valueType: 'select',
      valueEnum: Object.fromEntries(
        Object.entries(MISSING_ITEM_LABELS).map(([value, label]) => [value, { text: label }]),
      ),
    },
    {
      title: '截止状态',
      dataIndex: 'deadline_state',
      hideInTable: true,
      valueType: 'select',
      valueEnum: { open: { text: '未截止' }, closed: { text: '已截止' } },
    },
    { title: '选手姓名', dataIndex: 'player_name', width: 100 },
    {
      title: '组别·舞种',
      width: 140,
      render: (_, r) => `${r.level}·${DANCE_TYPE_LABELS[r.dance_type]}`,
    },
    { title: '联系电话', dataIndex: 'contact_phone', width: 130 },
    {
      title: '已传/总项',
      width: 90,
      render: (_, r) => `${r.completed}/${r.total}`,
    },
    {
      title: '缺失项',
      dataIndex: 'missing_items',
      render: (_, r) =>
        r.missing_items.map((m) => <Tag key={m} color="warning">{MISSING_ITEM_LABELS[m]}</Tag>),
    },
    {
      title: '资料截止',
      dataIndex: 'material_deadline',
      width: 150,
      render: (_, r) => formatDateTime(r.material_deadline),
    },
    {
      title: '最后上传',
      dataIndex: 'material_updated_at',
      width: 150,
      render: (_, r) => formatDateTime(r.material_updated_at),
    },
    {
      title: '已联系',
      dataIndex: 'contacted',
      width: 90,
      render: (_, r) =>
        r.contacted ? (
          <Tag color="success">✓ {formatDateTime(r.contacted_at)}</Tag>
        ) : (
          <Tag>未联系</Tag>
        ),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 160,
      render: (_, r) => (
        <Space size={4}>
          {access.canAdmin && !r.contacted && (
            <Typography.Link onClick={() => { setContactTarget(r); setContactNote(''); }}>
              标记已联系
            </Typography.Link>
          )}
          <Typography.Link onClick={() => history.push(`/order/detail/${r.order_no}`)}>
            查看报名
          </Typography.Link>
        </Space>
      ),
    },
  ];

  const handleExport = async () => {
    try {
      const values = (await formRef.current?.validateFields()) ?? {};
      await exportMissingMaterials({
        event_id: values.event_id,
        level: values.level as MissingMaterialEntry['level'],
        dance_type: values.dance_type as MissingMaterialEntry['dance_type'],
        missing_item: values.missing_item as MissingItem | undefined,
        deadline_state: values.deadline_state as 'open' | 'closed' | undefined,
        keyword: values.player_name as string | undefined,
      });
      message.success('导出已开始，请查看浏览器下载');
    } catch {
      message.error('导出失败，请稍后重试');
    }
  };

  return (
    <PageContainer title={false}>
      <ProTable<MissingMaterialEntry>
        headerTitle="未传齐资料名单"
        rowKey="registration_id"
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        form={{ initialValues: { event_id: urlEventId } }}
        search={{ labelWidth: 'auto' }}
        request={async (params) => {
          const data = await listMissingMaterials({
            event_id: params.event_id as number | undefined,
            level: params.level as 'U11' | 'U13' | 'U15' | undefined,
            dance_type: params.dance_type as 'classical' | 'folk' | undefined,
            missing_item: params.missing_item as MissingItem | undefined,
            deadline_state: params.deadline_state as 'open' | 'closed' | undefined,
            keyword: params.player_name as string | undefined,
            page: params.current,
            page_size: params.pageSize,
          });
          return { data: data.list, total: data.total, success: true };
        }}
        pagination={{ defaultPageSize: 20 }}
        toolBarRender={() =>
          access.canAdmin
            ? [
                <Button key="export" onClick={handleExport}>
                  导出催收名单
                </Button>,
              ]
            : []
        }
      />
      <Modal
        title={`标记已联系：${contactTarget?.player_name ?? ''}`}
        open={!!contactTarget}
        onCancel={() => setContactTarget(null)}
        onOk={async () => {
          if (!contactTarget) return;
          await markMaterialContacted(contactTarget.registration_id, contactNote || '已电话联系');
          message.success('已标记');
          setContactTarget(null);
          actionRef.current?.reload();
        }}
      >
        <Typography.Paragraph type="secondary">记录催收时间与备注，避免重复催。</Typography.Paragraph>
        <input
          style={{ width: '100%', padding: '8px 12px' }}
          placeholder="备注（如：已电话提醒，3 天内传）"
          value={contactNote}
          onChange={(e) => setContactNote(e.target.value)}
        />
      </Modal>
    </PageContainer>
  );
};

export default MaterialsList;
