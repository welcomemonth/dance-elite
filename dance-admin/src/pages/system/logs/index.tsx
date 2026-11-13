import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
import { Alert, Tag } from 'antd';
import React, { useRef } from 'react';
import { formatDateTime } from '@/services/youwumeng/format';
import { listLogs } from '@/services/youwumeng/system';
import type { LogEntry } from '@/services/youwumeng/typings';

const ACTION_LABELS: Record<string, string> = {
  'admin.login': '登录',
  'admin.change_password': '修改密码',
  'admin.season.create': '创建赛季',
  'admin.season.update': '修改赛季',
  'admin.event.create': '创建赛事',
  'admin.event.update': '修改赛事',
  'admin.event.publish': '发布赛事',
  'admin.event.offline': '下线赛事',
  'admin.event.close_registration': '关闭报名',
  'admin.event_group.quota': '调整名额',
  'admin.event_group.toggle': '关闭/重开组别',
  'admin.manual_registration.create': '人工报名',
  'admin.player.merge': '合并选手档案',
  'admin.refund.create': '登记退款',
  'admin.refund.approve': '同意退款',
  'admin.refund.reject': '驳回退款',
  'admin.refund.failed': '退款失败',
  'admin.cs_ticket.create': '登记客服工单',
  'admin.cs_ticket.update': '流转客服工单',
  'admin.waitlist.notify': '候补通知',
  'admin.waitlist.remove': '移出候补',
  'admin.waitlist.convert': '候补转报名',
  'admin.material.contacted': '资料催收标记',
  'admin.registration.export': '导出报名名单',
  audio_review_token: '生成审核链接',
  account_create: '创建账号',
  account_update: '修改账号',
  content_update: '修改内容',
  admin_login_failed: '登录失败',
};

const ACTION_COLOR: Record<string, string> = {
  manual_refund: 'volcano',
  reveal_phone: 'orange',
  correct_score: 'red',
  login: 'blue',
};

/** 操作日志（设计文档 10.2）：全量查询，不可删除 */
const Logs: React.FC = () => {
  const access = useAccess();
  const actionRef = useRef<ActionType>(null);

  const columns: ProColumns<LogEntry>[] = [
    { title: '时间', dataIndex: 'created_at', width: 170, render: (_, r) => formatDateTime(r.created_at) },
    {
      title: '操作人',
      dataIndex: 'admin_name',
      width: 120,
      search: false,
      render: (_, r) => r.admin_name,
    },
    {
      title: '动作',
      dataIndex: 'action',
      width: 150,
      valueEnum: Object.fromEntries(
        Object.entries(ACTION_LABELS).map(([value, text]) => [value, { text }]),
      ),
      render: (_, r) => (
        <Tag color={ACTION_COLOR[r.action] ?? 'default'}>{ACTION_LABELS[r.action] ?? r.action}</Tag>
      ),
    },
    { title: '对象', dataIndex: 'target_type', width: 200, render: (_, r) => `${r.target_type}#${r.target_id}` },
    { title: '详情', dataIndex: 'detail', ellipsis: true, render: (_, r) => JSON.stringify(r.detail) },
  ];

  if (!access.isSuperAdmin) {
    return (
      <PageContainer title={false}>
        <Alert type="warning" showIcon message="操作日志仅超级管理员可查看。" />
      </PageContainer>
    );
  }

  return (
    <PageContainer title={false}>
      <ProTable<LogEntry>
        headerTitle="操作日志"
        rowKey="log_id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto' }}
        request={async (params) => {
          const data = await listLogs({
            action: params.action as string | undefined,
            created_from: params.created_at ? String(params.created_at[0]) : undefined,
            created_to: params.created_at ? String(params.created_at[1]) : undefined,
            page: params.current,
            page_size: params.pageSize,
          });
          return { data: data.list, total: data.total, success: true };
        }}
        pagination={{ defaultPageSize: 20 }}
      />
    </PageContainer>
  );
};

export default Logs;
