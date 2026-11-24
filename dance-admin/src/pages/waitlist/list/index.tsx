import type { ActionType, ProColumns, ProFormInstance } from '@ant-design/pro-components';
import { PageContainer, ProFormSelect, ProTable } from '@ant-design/pro-components';
import { useAccess, useSearchParams } from '@umijs/max';
import { useService } from '@/services/youwumeng/useService';
import {
  Alert,
  App,
  Button,
  Form,
  Input,
  Modal,
  Radio,
  Space,
  Tag,
  Typography,
} from 'antd';
import React, { useRef, useState } from 'react';
import { formatDateTime } from '@/services/youwumeng/format';
import {
  AGE_LEVEL_OPTIONS,
  DANCE_TYPE_LABELS,
  WAITLIST_STATUS_LABELS,
} from '@/services/youwumeng/labels';
import { listEvents } from '@/services/youwumeng/events';
import {
  convertWaitlist,
  exportWaitlists,
  listWaitlists,
  notifyWaitlist,
  removeWaitlist,
} from '@/services/youwumeng/waitlists';
import type {
  AgeLevel,
  DanceType,
  WaitlistEntry,
  WaitlistStatus,
} from '@/services/youwumeng/typings';

const STATUS_COLORS: Record<WaitlistStatus, string> = {
  waiting: 'processing',
  notified: 'warning',
  converted: 'success',
  given_up: 'default',
  removed: 'default',
};

type ModalState =
  | { type: 'notify'; record: WaitlistEntry }
  | { type: 'remove'; record: WaitlistEntry }
  | { type: 'convert'; record: WaitlistEntry }
  | null;

const NOTIFY_OPTIONS = [
  { value: 'confirmed_intent', label: '确认有意向' },
  { value: 'no_answer', label: '无法接通' },
  { value: 'callback_later', label: '待回复' },
];

/**
 * 候补管理：按赛事+组别+舞种分组队列，按提交时间排序显示序号。
 * 对应设计文档 6.1/6.2 节。
 */
const WaitlistList: React.FC = () => {
  const { message } = App.useApp();
  const access = useAccess();
  const [searchParams] = useSearchParams();
  const urlEventId = searchParams.get('event_id')
    ? Number(searchParams.get('event_id'))
    : undefined;
  const actionRef = useRef<ActionType>(null);
  const formRef = useRef<ProFormInstance | undefined>(undefined);
  const [modal, setModal] = useState<ModalState>(null);
  const [modalValue, setModalValue] = useState('');
  const [releasedQuota, setReleasedQuota] = useState(0);

  const { data: eventOptions } = useService(async () => {
    const res = await listEvents({ page: 1, page_size: 100 });
    return res.list.map((e) => ({ label: e.name, value: e.id }));
  });

  const closeModal = () => {
    setModal(null);
    setModalValue('');
  };

  const handleExport = async () => {
    try {
      const values = (await formRef.current?.validateFields()) ?? {};
      await exportWaitlists({
        event_id: values.event_id,
        level: values.level as AgeLevel | undefined,
        dance_type: values.dance_type as DanceType | undefined,
        status: values.status as WaitlistStatus | undefined,
      });
      message.success('导出已开始，请查看浏览器下载');
    } catch {
      message.error('导出失败，请稍后重试');
    }
  };

  const handleNotify = async () => {
    if (!modal || modal.type !== 'notify') return;
    if (!modalValue) {
      message.warning('请选择通知结果');
      return;
    }
    await notifyWaitlist(modal.record.waitlist_id, modalValue);
    message.success('已标记通知');
    closeModal();
    actionRef.current?.reload();
  };

  const handleRemove = async () => {
    if (!modal || modal.type !== 'remove') return;
    if (!modalValue.trim()) {
      message.warning('请填写移出原因');
      return;
    }
    await removeWaitlist(modal.record.waitlist_id, modalValue.trim());
    message.success('已移出队列');
    closeModal();
    actionRef.current?.reload();
  };

  const handleConvert = async () => {
    if (!modal || modal.type !== 'convert') return;
    if (!modalValue.trim()) {
      message.warning('请填写订单号');
      return;
    }
    await convertWaitlist(modal.record.waitlist_id, modalValue.trim());
    message.success('已关联订单并转为报名');
    closeModal();
    actionRef.current?.reload();
  };

  const columns: ProColumns<WaitlistEntry>[] = [
    {
      title: '序号',
      dataIndex: 'position_no',
      width: 70,
      search: false,
      render: (_, record) => <Tag>{record.position_no}</Tag>,
    },
    {
      title: '选手姓名',
      dataIndex: 'name',
      width: 100,
    },
    {
      title: '证件号',
      dataIndex: 'id_card_masked',
      width: 190,
      search: false,
      render: (_, record) => record.id_card_masked ?? <Tag>无证件号</Tag>,
    },
    { title: '机构', dataIndex: 'institution', width: 130, search: false, render: (v) => v || '-' },
    { title: '联系电话', dataIndex: 'phone', width: 120, search: false },
    { title: '赛事', dataIndex: 'event_name', width: 200, search: false },
    {
      title: '赛事筛选',
      dataIndex: 'event_id',
      hideInTable: true,
      valueType: 'select',
      request: async () => eventOptions ?? [],
    },
    {
      title: '组别',
      dataIndex: 'level',
      width: 80,
      valueEnum: Object.fromEntries(AGE_LEVEL_OPTIONS.map((l) => [l, { text: l }])),
    },
    {
      title: '舞种',
      dataIndex: 'dance_type',
      width: 110,
      valueEnum: Object.fromEntries(
        Object.entries(DANCE_TYPE_LABELS).map(([value, label]) => [value, { text: label }]),
      ),
    },
    {
      title: '提交时间',
      dataIndex: 'created_at',
      width: 150,
      search: false,
      render: (_, record) => formatDateTime(record.created_at),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      valueEnum: Object.fromEntries(
        Object.entries(WAITLIST_STATUS_LABELS).map(([value, text]) => [value, { text }]),
      ),
      render: (_, record) => (
        <Tag color={STATUS_COLORS[record.status]}>
          {WAITLIST_STATUS_LABELS[record.status]}
        </Tag>
      ),
    },
    {
      title: '通知时间',
      dataIndex: 'notified_at',
      width: 150,
      search: false,
      render: (_, record) => formatDateTime(record.notified_at),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 190,
      render: (_, record) =>
        access.canAdmin && (
          <Space size={4} wrap>
            {record.status === 'waiting' && (
              <Typography.Link onClick={() => setModal({ type: 'notify', record })}>
                标记已通知
              </Typography.Link>
            )}
            {(record.status === 'waiting' || record.status === 'notified') && (
              <>
                <Typography.Link onClick={() => setModal({ type: 'convert', record })}>
                  关联订单转报名
                </Typography.Link>
                <Typography.Link type="danger" onClick={() => setModal({ type: 'remove', record })}>
                  移出队列
                </Typography.Link>
              </>
            )}
          </Space>
        ),
    },
  ];

  return (
    <PageContainer title={false}>
      {releasedQuota > 0 && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          title={`该组别已释放 ${releasedQuota} 个名额，请按序号电话联系候补者（名额在小程序端先到先得，候补仅作兜底）`}
        />
      )}
      <ProTable<WaitlistEntry>
        headerTitle="候补队列"
        rowKey="waitlist_id"
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        form={{ initialValues: { event_id: urlEventId } }}
        search={{ labelWidth: 'auto' }}
        request={async (params) => {
          const data = await listWaitlists({
            event_id: params.event_id,
            level: params.level as AgeLevel | undefined,
            dance_type: params.dance_type as DanceType | undefined,
            status: params.status as WaitlistStatus | undefined,
            page: params.current,
            page_size: params.pageSize,
          });
          setReleasedQuota(data.released_quota);
          return { data: data.list, total: data.total, success: true };
        }}
        pagination={{ defaultPageSize: 20 }}
        toolBarRender={() =>
          access.canAdmin
            ? [
                <Button key="export" onClick={handleExport}>
                  导出候补队列
                </Button>,
              ]
            : []
        }
      />

      <Modal
        title={`标记已通知 - ${modal?.type === 'notify' ? modal.record.name : ''}`}
        open={modal?.type === 'notify'}
        onOk={handleNotify}
        onCancel={closeModal}
        okText="确认"
        cancelText="取消"
      >
        <Form layout="vertical">
          <Form.Item label="通知结果" required>
            <Radio.Group
              options={NOTIFY_OPTIONS}
              value={modalValue}
              onChange={(e) => setModalValue(e.target.value as string)}
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`移出队列 - ${modal?.type === 'remove' ? modal.record.name : ''}`}
        open={modal?.type === 'remove'}
        onOk={handleRemove}
        onCancel={closeModal}
        okText="确认移出"
        okButtonProps={{ danger: true }}
        cancelText="取消"
      >
        <Input.TextArea
          rows={3}
          placeholder="移出原因（放弃/多次联系不上/不符合条件）"
          value={modalValue}
          onChange={(e) => setModalValue(e.target.value)}
        />
      </Modal>

      <Modal
        title={`关联订单转报名 - ${modal?.type === 'convert' ? modal.record.name : ''}`}
        open={modal?.type === 'convert'}
        onOk={handleConvert}
        onCancel={closeModal}
        okText="确认"
        cancelText="取消"
      >
        <Input
          placeholder="用户完成支付后的订单号"
          value={modalValue}
          onChange={(e) => setModalValue(e.target.value)}
        />
      </Modal>
    </PageContainer>
  );
};

export default WaitlistList;
