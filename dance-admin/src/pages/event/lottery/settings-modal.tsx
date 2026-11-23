import { App, DatePicker, Descriptions, Form, Input, InputNumber, Modal, Table, Typography } from 'antd';
import type { Dayjs } from 'dayjs';
import dayjs from 'dayjs';
import React, { useEffect, useState } from 'react';
import { getLotterySession, updateLotterySession } from '@/services/youwumeng/lottery';
import type { LotteryGroupStat, LotterySessionDetail } from '@/services/youwumeng/typings';

interface SettingsFormValues {
  start_at?: Dayjs;
  end_at?: Dayjs;
  remind_before_minutes?: number;
  sms_remind_template?: string;
  sms_result_template?: string;
}

/** 「抽签设置」弹窗：维护时间窗口/短信模板，并展示分组进度。 */
const SettingsLotteryModal: React.FC<{
  token: string | null;
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}> = ({ token, open, onClose, onSuccess }) => {
  const { message } = App.useApp();
  const [form] = Form.useForm<SettingsFormValues>();
  const [detail, setDetail] = useState<LotterySessionDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!open || !token) return;
    setLoading(true);
    getLotterySession(token)
      .then((data) => {
        setDetail(data);
        form.setFieldsValue({
          start_at: data.start_at ? dayjs(data.start_at) : undefined,
          end_at: data.end_at ? dayjs(data.end_at) : undefined,
          remind_before_minutes: data.remind_before_minutes || 120,
          sms_remind_template: data.sms_remind_template,
          sms_result_template: data.sms_result_template,
        });
      })
      .catch((err) => message.error(err instanceof Error ? err.message : '加载失败'))
      .finally(() => setLoading(false));
  }, [open, token, form, message]);

  const close = () => {
    setDetail(null);
    form.resetFields();
    onClose();
  };

  const handleSubmit = async () => {
    if (!token) return;
    const values = await form.validateFields();
    setSubmitting(true);
    try {
      await updateLotterySession(token, {
        start_at: values.start_at?.format('YYYY-MM-DD HH:mm') ?? '',
        end_at: values.end_at?.format('YYYY-MM-DD HH:mm') ?? '',
        remind_before_minutes: values.remind_before_minutes ?? 120,
        sms_remind_template: values.sms_remind_template ?? '',
        sms_result_template: values.sms_result_template ?? '',
      });
      message.success('抽签设置已保存');
      onSuccess();
      close();
    } catch (err) {
      message.error(err instanceof Error ? err.message : '保存失败，请稍后重试');
    } finally {
      setSubmitting(false);
    }
  };

  const groupColumns = [
    { title: '舞种', dataIndex: 'genre_label', width: 120 },
    { title: '年龄组', dataIndex: 'age_group', width: 90 },
    { title: '分组', dataIndex: 'letter', width: 80, render: (letter: string) => `${letter} 组` },
    { title: '人数', dataIndex: 'size', width: 80 },
    {
      title: '已抽',
      dataIndex: 'drawn',
      width: 80,
      render: (_: number, record: LotteryGroupStat) => `${record.drawn}/${record.size}`,
    },
  ];

  return (
    <Modal
      title="抽签设置"
      open={open}
      onCancel={close}
      onOk={handleSubmit}
      okText="保存"
      confirmLoading={submitting}
      loading={loading}
      width={720}
      destroyOnHidden
    >
      {detail && (
        <>
          <Descriptions column={1} size="small" style={{ marginBottom: 16 }}>
            <Descriptions.Item label="赛事">{detail.event_name}</Descriptions.Item>
            <Descriptions.Item label="抽签链接">
              <Typography.Text copyable={{ text: detail.link }}>
                {detail.link}
              </Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label="进度">
              已抽 {detail.drawn_count} / 共 {detail.player_count} 人
              {detail.pending_count > 0 ? `（未抽 ${detail.pending_count} 人）` : ''}
            </Descriptions.Item>
          </Descriptions>

          <Form form={form} layout="vertical">
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
            <Form.Item name="remind_before_minutes" label="截止前提醒短信（分钟）">
              <InputNumber min={0} max={10080} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="sms_remind_template" label="提醒短信模板 CODE">
              <Input placeholder="留空则不发提醒短信" />
            </Form.Item>
            <Form.Item name="sms_result_template" label="结果短信模板 CODE">
              <Input placeholder="留空则不发结果短信" />
            </Form.Item>
          </Form>

          <Table<LotteryGroupStat>
            rowKey={(record) => `${record.genre}-${record.age_group}-${record.letter}`}
            columns={groupColumns}
            dataSource={detail.groups}
            pagination={false}
            size="small"
          />
        </>
      )}
    </Modal>
  );
};

export default SettingsLotteryModal;
