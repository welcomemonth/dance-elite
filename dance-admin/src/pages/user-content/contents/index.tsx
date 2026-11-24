import { PageContainer } from '@ant-design/pro-components';
import { useSearchParams } from '@umijs/max';
import {
  Alert,
  App,
  Button,
  Card,
  DatePicker,
  Flex,
  Form,
  Image,
  Input,
  Menu,
  Space,
  Upload,
} from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useState } from 'react';
import { getContent, putContent, uploadContentImage } from '@/services/youwumeng/contents';
import type { ContentKey } from '@/services/youwumeng/typings';

const KEYS: { key: ContentKey; label: string }[] = [
  { key: 'points_rules_desc', label: '积分规则说明' },
  { key: 'event_faq', label: '赛事规则 / FAQ' },
  { key: 'official_account', label: '公众号配置' },
  { key: 'announcement', label: '公告' },
];

/** 内容管理（设计文档 9.2）：小程序端展示内容的键值维护 */
const Contents: React.FC = () => {
  const { message } = App.useApp();
  const [searchParams] = useSearchParams();
  const [active, setActive] = useState<ContentKey>(
    (searchParams.get('key') as ContentKey) || 'points_rules_desc',
  );
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setLoading(true);
    getContent(active)
      .then((entry) => {
        form.setFieldsValue({
          text: entry.value.text,
          image_url: entry.value.image_url,
          range: [
            entry.value.online_at ? dayjs(entry.value.online_at) : null,
            entry.value.offline_at ? dayjs(entry.value.offline_at) : null,
          ],
        });
      })
      .finally(() => setLoading(false));
  }, [active, form]);

  return (
    <PageContainer title={false}>
      <Card
        size="small"
        title="内容管理"
        extra={
          <Alert
            type="info"
            showIcon
            title="小程序端对应页面从同一份配置读取；修改记操作日志。"
          />
        }
      >
        <Space align="start" size={24}>
          <Menu
            mode="inline"
            selectedKeys={[active]}
            style={{ width: 180 }}
            items={KEYS.map((k) => ({ key: k.key, label: k.label }))}
            onClick={({ key }) => setActive(key as ContentKey)}
          />
          <Card
            loading={loading}
            style={{ minWidth: 560 }}
            title={KEYS.find((k) => k.key === active)?.label}
            extra={
              <Button
                type="primary"
                loading={saving}
                onClick={async () => {
                  const values = await form.validateFields();
                  setSaving(true);
                  try {
                    await putContent(active, {
                      text: values.text,
                      image_url: values.image_url || undefined,
                      online_at: values.range?.[0]?.toISOString(),
                      offline_at: values.range?.[1]?.toISOString(),
                    });
                    message.success('已保存');
                  } finally {
                    setSaving(false);
                  }
                }}
              >
                保存
              </Button>
            }
          >
            <Form form={form} layout="vertical">
              <Form.Item
                name="text"
                label="文案内容"
                rules={[{ required: true, message: '请输入文案' }]}
              >
                <Input.TextArea rows={active === 'event_faq' ? 12 : 6} />
              </Form.Item>
              {active === 'official_account' && (
                <Form.Item label="公众号二维码图片">
                  <Flex vertical gap={12}>
                    <Upload
                      accept="image/png,image/jpeg"
                      showUploadList={false}
                      customRequest={async ({ file, onSuccess, onError }) => {
                        try {
                          const entry = await uploadContentImage('official_account', file as File);
                          form.setFieldValue('image_url', entry.value.image_url);
                          message.success('二维码已上传并保存');
                          onSuccess?.(entry);
                        } catch (error) {
                          message.error('上传失败，请稍后重试');
                          onError?.(error as Error);
                        }
                      }}
                    >
                      <Button>上传二维码图片（png/jpg，≤2MB）</Button>
                    </Upload>
                    <Form.Item noStyle shouldUpdate>
                      {({ getFieldValue }) =>
                        getFieldValue('image_url') ? (
                          <Image
                            width={160}
                            src={getFieldValue('image_url')}
                            alt="公众号二维码预览"
                          />
                        ) : null
                      }
                    </Form.Item>
                  </Flex>
                  <Form.Item name="image_url" hidden>
                    <Input />
                  </Form.Item>
                </Form.Item>
              )}
              {active === 'announcement' && (
                <Form.Item name="range" label="上线 / 下线时间">
                  <DatePicker.RangePicker showTime style={{ width: '100%' }} />
                </Form.Item>
              )}
            </Form>
          </Card>
        </Space>
      </Card>
    </PageContainer>
  );
};

export default Contents;
