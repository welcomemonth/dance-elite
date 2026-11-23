import { PageContainer, ProForm, ProFormText } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { App, Button, Card, Space } from 'antd';
import React, { useState } from 'react';
import { changePassword } from '@/services/youwumeng/auth';

interface FormValues {
  old_password: string;
  new_password: string;
  confirm_password: string;
}

/** 当前管理员修改密码（PUT /api/v1/admin/auth/password）。 */
const ChangePassword: React.FC = () => {
  const { message } = App.useApp();
  const [submitting, setSubmitting] = useState(false);

  const onFinish = async (values: FormValues) => {
    setSubmitting(true);
    try {
      await changePassword({
        old_password: values.old_password,
        new_password: values.new_password,
      });
      message.success('密码已修改，请牢记新密码');
      history.back();
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <PageContainer
      title="修改密码"
      header={{
        breadcrumb: {
          items: [{ title: '首页', path: '/dashboard' }, { title: '修改密码' }],
        },
      }}
    >
      <Card style={{ maxWidth: 520 }}>
        <ProForm<FormValues>
          layout="vertical"
          submitter={false}
          onFinish={onFinish}
        >
          <ProFormText.Password
            name="old_password"
            label="当前密码"
            rules={[{ required: true, message: '请输入当前密码' }]}
          />
          <ProFormText.Password
            name="new_password"
            label="新密码"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 8, message: '新密码至少 8 位' },
            ]}
          />
          <ProFormText.Password
            name="confirm_password"
            label="确认新密码"
            dependencies={['new_password']}
            rules={[
              { required: true, message: '请再次输入新密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('new_password') === value) {
                    return Promise.resolve();
                  }
                  return Promise.reject(new Error('两次输入的新密码不一致'));
                },
              }),
            ]}
          />
          <Space>
            <Button type="primary" htmlType="submit" loading={submitting}>
              保存
            </Button>
            <Button onClick={() => history.back()}>取消</Button>
          </Space>
        </ProForm>
      </Card>
    </PageContainer>
  );
};

export default ChangePassword;
