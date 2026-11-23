import { InboxOutlined } from '@ant-design/icons';
import { Alert, App, DatePicker, Form, Input, InputNumber, Modal, Segmented, Select, Switch, Upload } from 'antd';
import type { Dayjs } from 'dayjs';
import dayjs from 'dayjs';
import React, { useEffect, useState } from 'react';
import { listEvents } from '@/services/youwumeng/events';
import { importLotterySession } from '@/services/youwumeng/lottery';
import type { EventBrief } from '@/services/youwumeng/typings';

interface ImportFormValues {
  event_id?: number;
  title?: string;
  start_at?: Dayjs;
  end_at?: Dayjs;
  remind_before_minutes?: number;
  sms_remind_template?: string;
  sms_result_template?: string;
  replace?: boolean;
}

/** 「导入抽签」弹窗：选择赛事 + 上传报名 Excel + 抽签时间窗口。 */
const ImportLotteryModal: React.FC<{
  open: boolean;
  onClose: () => void;
  onSuccess: (link: string) => void;
}> = ({ open, onClose, onSuccess }) => {
  const { message } = App.useApp();
  const [form] = Form.useForm<ImportFormValues>();
  const [events, setEvents] = useState<EventBrief[]>([]);
  const [file, setFile] = useState<File | null>(null);
  const [source, setSource] = useState<'excel' | 'event'>('excel');
  const [submitting, setSubmitting] = useState(false);
  const selectedEventId = Form.useWatch('event_id', form);
  const needTitle = source === 'excel' && !selectedEventId;

  useEffect(() => {
    if (!open) return;
    listEvents({ page_size: 100 })
      .then((data) => setEvents(data.list))
      .catch(() => setEvents([]));
  }, [open]);

  const reset = () => {
    form.resetFields();
    setFile(null);
    setSource('excel');
  };

  const close = () => {
    reset();
    onClose();
  };

  // 选中赛事后，用赛事时间线的抽签时间预填开始时间，结束时间默认两天后。
  const handleEventChange = (eventId?: number) => {
    const event = eventId ? events.find((item) => item.id === eventId) : undefined;
    const start = event?.lottery_at ? dayjs(event.lottery_at) : null;
    if (start?.isValid()) {
      form.setFieldValue('start_at', start);
      form.setFieldValue('end_at', start.add(2, 'day'));
    }
    if (event && !form.getFieldValue('title')) {
      form.setFieldValue('title', event.name);
    }
  };

  // 从赛事报名生成时只展示报名已结束的赛事（后端仍会二次校验）。
  const eventOptions = events
    .filter(
      (item) =>
        source === 'excel' ||
        ['reg_closed', 'ongoing', 'finished'].includes(item.status),
    )
    .map((item) => ({ value: item.id, label: item.name }));

  const handleSubmit = async () => {
    const values = await form.validateFields();
    if (source === 'excel' && !file) {
      message.error('请上传报名 Excel 文件（.xlsx）');
      return;
    }
    const formData = new FormData();
    if (values.event_id) {
      formData.append('event_id', String(values.event_id));
    }
    if (source === 'excel' && values.title) {
      formData.append('title', values.title.trim());
    }
    formData.append('source', source);
    if (source === 'excel' && file) {
      formData.append('file', file, file.name);
    }
    if (values.start_at) {
      formData.append('start_at', values.start_at.format('YYYY-MM-DD HH:mm'));
    }
    if (values.end_at) {
      formData.append('end_at', values.end_at.format('YYYY-MM-DD HH:mm'));
    }
    formData.append('remind_before_minutes', String(values.remind_before_minutes ?? 120));
    formData.append('replace', values.replace ? 'true' : 'false');
    if (values.sms_remind_template) {
      formData.append('sms_remind_template', values.sms_remind_template);
    }
    if (values.sms_result_template) {
      formData.append('sms_result_template', values.sms_result_template);
    }

    setSubmitting(true);
    try {
      const result = await importLotterySession(formData);
      message.success(`已生成抽签：${result.player_count} 名选手`);
      reset();
      onSuccess(result.link);
    } catch (err) {
      message.error(err instanceof Error ? err.message : '导入失败，请稍后重试');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="生成赛事抽签"
      open={open}
      onCancel={close}
      onOk={handleSubmit}
      okText="生成抽签链接"
      confirmLoading={submitting}
      width={640}
      destroyOnHidden
    >
      <Form
        form={form}
        layout="vertical"
        initialValues={{ remind_before_minutes: 120, replace: false }}
      >
        <Form.Item label="抽签数据来源">
          <Segmented
            block
            value={source}
            onChange={(value) => {
              setSource(value as 'excel' | 'event');
              setFile(null);
              form.setFieldValue('event_id', undefined);
            }}
            options={[
              { label: '上传 Excel 解析', value: 'excel' },
              { label: '使用赛事报名结果', value: 'event' },
            ]}
          />
        </Form.Item>
        <Form.Item
          name="event_id"
          label={source === 'excel' ? '关联赛事（可选）' : '赛事'}
          rules={
            source === 'event'
              ? [{ required: true, message: '从赛事报名生成时必须选择赛事' }]
              : []
          }
          tooltip="上传 Excel 时可以不关联赛事，只生成抽签链接；不关联时请在下方填写抽签名称。"
        >
          <Select
            allowClear
            showSearch={{ optionFilterProp: 'label' }}
            placeholder={
              source === 'event' ? '选择报名已结束的赛事' : '可不选，仅用 Excel 生成抽签链接'
            }
            options={eventOptions}
            notFoundContent={
              source === 'event' ? '暂无报名已结束的赛事' : '暂无赛事，仍可不关联直接生成'
            }
            onChange={handleEventChange}
          />
        </Form.Item>

        {source === 'excel' ? (
          <>
            <Form.Item
              name="title"
              label="抽签名称"
              rules={[
                {
                  validator(_, value: string | undefined) {
                    if (!needTitle || (value ?? '').trim()) {
                      return Promise.resolve();
                    }
                    return Promise.reject(new Error('未选择赛事时请填写抽签名称'));
                  },
                },
              ]}
            >
              <Input
                placeholder={needTitle ? '如：2027 春季班汇报抽签（未关联赛事时必填）' : '留空默认使用赛事名称'}
                maxLength={100}
              />
            </Form.Item>
            <Form.Item label="报名 Excel（.xlsx）" required>
            <Upload.Dragger
            accept=".xlsx"
            maxCount={1}
            fileList={file ? [{ uid: '-1', name: file.name, status: 'done' as const }] : []}
            beforeUpload={(selected) => {
              setFile(selected as File);
              return false;
            }}
            onRemove={() => setFile(null)}
            onChange={({ fileList }) => {
              if (fileList.length === 0) setFile(null);
            }}
          >
            <p className="ant-upload-drag-icon">
              <InboxOutlined />
            </p>
            <p className="ant-upload-text">点击或拖拽报名表到此区域</p>
              <p className="ant-upload-hint">
                需包含「姓名 / 身份证号 / 组别及场次」列；舞种、剧目、机构、手机号为可选列。
              </p>
              </Upload.Dragger>
            </Form.Item>
          </>
        ) : (
          <Alert
            type="info"
            showIcon
            title="将读取该赛事已截止报名后的有效报名（已支付）记录自动分组，无需上传文件。"
            description="若赛事报名尚未结束或暂无有效报名，会提示无法生成。"
            style={{ marginBottom: 16 }}
          />
        )}

        <Form.Item name="start_at" label="抽签开始时间（留空表示不限制）">
          <DatePicker showTime format="YYYY-MM-DD HH:mm" style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item
          name="end_at"
          label="抽签截止时间（到点后未抽选手由系统自动分配）"
          dependencies={['start_at']}
          rules={[
            ({ getFieldValue }) => ({
              validator(_, value: Dayjs | undefined) {
                const start = getFieldValue('start_at') as Dayjs | undefined;
                if (!value || !start || start.isBefore(value)) {
                  return Promise.resolve();
                }
                return Promise.reject(new Error('截止时间必须晚于开始时间'));
              },
            }),
          ]}
        >
          <DatePicker showTime format="YYYY-MM-DD HH:mm" style={{ width: '100%' }} />
        </Form.Item>

        <Form.Item
          name="remind_before_minutes"
          label="截止前提醒短信（分钟，默认 120）"
          tooltip="截止前该时长内给未抽签选手发送提醒短信；需配置短信模板。"
        >
          <InputNumber min={0} max={10080} style={{ width: '100%' }} />
        </Form.Item>

        <Form.Item name="sms_remind_template" label="提醒短信模板 CODE（可选）">
          <Input placeholder="如 SMS_123456789，留空则不发提醒短信" />
        </Form.Item>
        <Form.Item name="sms_result_template" label="结果短信模板 CODE（可选）">
          <Input placeholder="如 SMS_987654321，留空则不发结果短信" />
        </Form.Item>

        {selectedEventId ? (
          <Form.Item
            name="replace"
            label="覆盖该赛事已有抽签"
            valuePropName="checked"
            tooltip="覆盖会清空已有抽签结果与短信记录，但沿用原链接。"
          >
            <Switch />
          </Form.Item>
        ) : (
          <Alert
            type="warning"
            showIcon
            title="未关联赛事：每次生成都会创建新的抽签链接，不会覆盖已有抽签。"
            style={{ marginBottom: 8 }}
          />
        )}
      </Form>
    </Modal>
  );
};

export default ImportLotteryModal;
