import type { ActionType, ProColumns, ProFormInstance } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { history, useAccess, useSearchParams } from '@umijs/max';
import { App, Button, Space, Switch, Tag, Typography } from 'antd';
import React, { useRef, useState } from 'react';
import { formatDateTime, formatMoney } from '@/services/youwumeng/format';
import {
  AGE_LEVEL_OPTIONS,
  DANCE_TYPE_LABELS,
  ORDER_SOURCE_LABELS,
  ORDER_STATUS_LABELS,
} from '@/services/youwumeng/labels';
import { listOrders, exportOrders } from '@/services/youwumeng/orders';
import { listEvents } from '@/services/youwumeng/events';
import type {
  AgeLevel,
  DanceType,
  OrderBrief,
  OrderSource,
  OrderStatus,
} from '@/services/youwumeng/typings';

const STATUS_COLORS: Record<OrderStatus, string> = {
  pending_payment: 'default',
  paid: 'success',
  canceled: 'default',
  refunding: 'processing',
  refunded: 'warning',
};

const OrderList: React.FC = () => {
  const actionRef = useRef<ActionType>(null);
  const formRef = useRef<ProFormInstance>(undefined);
  const access = useAccess();
  const { message } = App.useApp();
  const [searchParams] = useSearchParams();
  const urlEventId = searchParams.get('event_id')
    ? Number(searchParams.get('event_id'))
    : undefined;
  const [includeCanceled, setIncludeCanceled] = useState(false);

  const handleExport = async () => {
    try {
      const values = (await formRef.current?.validateFields()) ?? {};
      const paidRange = values.paid_range as [string, string] | undefined;
      await exportOrders({
        event_id: values.event_id,
        level: values.level,
        dance_type: values.dance_type,
        status: values.status,
        source: values.source,
        include_canceled: includeCanceled,
        paid_from: paidRange?.[0],
        paid_to: paidRange?.[1],
        keyword: values.order_no || values.player_name,
      });
      message.success('导出已开始，请查看浏览器下载');
    } catch {
      message.error('导出失败，请稍后重试');
    }
  };

  const columns: ProColumns<OrderBrief>[] = [
    {
      title: '订单号',
      dataIndex: 'order_no',
      width: 200,
      render: (_, record) => (
        <Typography.Link
          onClick={() => history.push(`/order/detail/${record.order_no}`)}
        >
          {record.order_no}
        </Typography.Link>
      ),
    },
    { title: '赛事', dataIndex: 'event_name', width: 240, search: false },
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
      title: '组别',
      dataIndex: 'level',
      width: 70,
      valueEnum: Object.fromEntries(
        AGE_LEVEL_OPTIONS.map((lv) => [lv, { text: lv }]),
      ),
    },
    {
      title: '舞种',
      dataIndex: 'dance_type',
      width: 100,
      valueEnum: Object.fromEntries(
        Object.entries(DANCE_TYPE_LABELS).map(([value, text]) => [value, { text }]),
      ),
      render: (_, record) => DANCE_TYPE_LABELS[record.dance_type],
    },
    { title: '选手姓名', dataIndex: 'player_name', width: 100 },
    {
      title: '选手编号',
      dataIndex: 'player_no',
      width: 110,
      search: false,
    },
    {
      title: '金额',
      dataIndex: 'amount',
      width: 110,
      search: false,
      render: (_, record) => formatMoney(record.amount),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 110,
      valueEnum: {
        paid: { text: ORDER_STATUS_LABELS.paid },
        canceled: { text: ORDER_STATUS_LABELS.canceled },
        refunding: { text: ORDER_STATUS_LABELS.refunding },
        refunded: { text: ORDER_STATUS_LABELS.refunded },
      },
      render: (_, record) => (
        <Tag color={STATUS_COLORS[record.status]}>
          {ORDER_STATUS_LABELS[record.status]}
          {record.status === 'canceled' ? '（留档）' : ''}
        </Tag>
      ),
    },
    {
      title: '来源',
      dataIndex: 'source',
      width: 90,
      valueEnum: Object.fromEntries(
        Object.entries(ORDER_SOURCE_LABELS).map(([value, text]) => [value, { text }]),
      ),
      render: (_, record) => ORDER_SOURCE_LABELS[record.source],
    },
    {
      title: '支付时间',
      dataIndex: 'paid_at',
      width: 150,
      search: false,
      render: (_, record) => formatDateTime(record.paid_at),
    },
    {
      title: '支付时间',
      dataIndex: 'paid_range',
      hideInTable: true,
      valueType: 'dateTimeRange',
      search: { transform: (value) => ({ paid_from: value?.[0], paid_to: value?.[1] }) },
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      width: 150,
      search: false,
      render: (_, record) => formatDateTime(record.created_at),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 140,
      render: (_, record) => (
        <Space size={4}>
          <Typography.Link
            onClick={() => history.push(`/order/detail/${record.order_no}`)}
          >
            查看详情
          </Typography.Link>
          {record.status === 'paid' && (
            <Typography.Link
              onClick={() =>
                history.push(`/order/refunds?keyword=${record.order_no}`)
              }
            >
              发起退款
            </Typography.Link>
          )}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer title={false}>
      <ProTable<OrderBrief>
        headerTitle="订单管理"
        rowKey="order_no"
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        form={{ initialValues: { event_id: urlEventId } }}
        search={{ labelWidth: 'auto' }}
        request={async (params) => {
          const paidRange = params.paid_range as [string, string] | undefined;
          const data = await listOrders({
            event_id: params.event_id as number | undefined,
            level: params.level as AgeLevel | undefined,
            dance_type: params.dance_type as DanceType | undefined,
            status: params.status as OrderStatus | undefined,
            source: params.source as OrderSource | undefined,
            include_canceled: includeCanceled,
            paid_from: paidRange?.[0],
            paid_to: paidRange?.[1],
            keyword: params.order_no || params.player_name,
            page: params.current,
            page_size: params.pageSize,
          });
          return { data: data.list, total: data.total, success: true };
        }}
        pagination={{ defaultPageSize: 20 }}
        toolBarRender={() => [
          ...(access.canAdmin
            ? [
                <Button key="export" onClick={handleExport}>
                  导出订单
                </Button>,
              ]
            : []),
          <span key="canceled">
            显示已取消（对账）
            <Switch
              style={{ marginLeft: 8, marginRight: 16 }}
              checked={includeCanceled}
              onChange={(checked) => {
                setIncludeCanceled(checked);
                actionRef.current?.reload();
              }}
            />
          </span>,
        ]}
      />
    </PageContainer>
  );
};

export default OrderList;
