
import type { ProColumns } from '@ant-design/pro-components';
import {
  PageContainer,
  ProFormList,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProTable,
  StepsForm,
} from '@ant-design/pro-components';
import {
  Alert,
  App,
  Button,
  Card,
  Form,
  Input,
  Modal,
  Select,
  Space,
  Typography,
  Upload,
} from 'antd';
import React, { useState } from 'react';
import { listEvents } from '@/services/youwumeng/events';
import { AWARD_OPTIONS, boardLabel } from '@/services/youwumeng/labels';
import {
  correctScore,
  downloadScoreTemplate,
  importScoresExcel,
  listScoreEntries,
  listSeasons,
  previewScores,
  publishScores,
} from '@/services/youwumeng/seasons';
import type {
  AgeLevel,
  DanceType,
  ScoreEntry as ScoreEntryRow,
  ScorePreview,
} from '@/services/youwumeng/typings';

const BOARDS: { level: AgeLevel; dance_type: DanceType }[] = [
  { level: 'U11', dance_type: 'classical' },
  { level: 'U11', dance_type: 'folk' },
  { level: 'U13', dance_type: 'classical' },
  { level: 'U13', dance_type: 'folk' },
  { level: 'U15', dance_type: 'classical' },
  { level: 'U15', dance_type: 'folk' },
];

/** 成绩录入向导（设计文档 8.2）：选择赛事与榜单 → 录入/导入 → 确认发布 */
const ScoreEntry: React.FC = () => {
  const { message } = App.useApp();
  const [_seasons, setSeasons] = useState<{ label: string; value: number }[]>(
    [],
  );
  const [eventOptions, setEventOptions] = useState<
    { label: string; value: number }[]
  >([]);
  const [selection, setSelection] = useState<{
    eventId: number;
    level: AgeLevel;
    danceType: DanceType;
  } | null>(null);
  const [preview, setPreview] = useState<ScorePreview | null>(null);
  const [importBatch, setImportBatch] = useState<string | undefined>();
  const [importing, setImporting] = useState(false);
  const [correctTarget, setCorrectTarget] = useState<ScoreEntryRow | null>(null);
  const [correctForm] = Form.useForm<{ award?: string; remark?: string }>();

  const handleUpload = async (file: File) => {
    if (!selection) {
      message.warning('请先完成上一步，选择赛事与榜单');
      return false;
    }
    setImporting(true);
    try {
      const result = await importScoresExcel(
        selection.eventId,
        selection.level,
        selection.danceType,
        file,
      );
      setPreview(result);
      setImportBatch(result.import_batch);
      message.success(
        `已解析：匹配 ${result.matched}，新建 ${result.created}，错误 ${result.errors}`,
      );
    } finally {
      setImporting(false);
    }
    return false;
  };

  const previewColumns: ProColumns<ScorePreview['rows'][number]>[] = [
    { title: '行号', dataIndex: 'row_no', width: 70 },
    { title: '姓名', dataIndex: 'player_name', width: 120 },
    { title: '证件号', dataIndex: 'id_card_masked', width: 180 },
    {
      title: '匹配',
      dataIndex: 'match_status',
      width: 110,
      render: (_, r) => {
        switch (r.match_status) {
          case 'matched':
            return <Typography.Text type="success">已匹配</Typography.Text>;
          case 'created':
            return <Typography.Text type="success">已新建</Typography.Text>;
          case 'new':
            return <Typography.Text type="warning">待建档</Typography.Text>;
          default:
            return <Typography.Text type="danger">错误</Typography.Text>;
        }
      },
    },
    { title: '奖项', dataIndex: 'award', render: (_, r) => AWARD_OPTIONS.find((a) => a.value === r.award)?.label ?? '-' },
    {
      title: '晋级决赛',
      dataIndex: 'advanced_to_final',
      render: (_, r) => (r.advanced_to_final ? '是' : '-'),
    },
    { title: '备注', dataIndex: 'remark' },
    { title: '错误原因', dataIndex: 'error', render: (_, r) => r.error ?? '-' },
  ];

  return (
    <PageContainer title="成绩录入">
      <Card>
        <StepsForm
          onFinish={async () => {
            if (!selection || !preview) return false;
            if (preview.errors > 0) {
              message.error('存在无法匹配/建档的行，请修正 Excel 后重新导入');
              return false;
            }
            await publishScores(
              selection.eventId,
              selection.level,
              selection.danceType,
              preview.rows,
              importBatch,
            );
            message.success('已发布，小程序端榜单即时更新');
            return true;
          }}
          submitter={{ render: (_, dom) => dom }}
        >
          <StepsForm.StepForm
            title="选择赛事与榜单"
            onFinish={async (values) => {
              const seasonId = values.season_id as number;
              const list = await listEvents({ page: 1, page_size: 100 });
              setEventOptions(
                list.list
                  .filter((e) => e.season_id === seasonId)
                  .map((e) => ({ label: e.name, value: e.id })),
              );
              const board = BOARDS[values.board_index as number];
              setSelection({
                eventId: values.event_id as number,
                level: board.level,
                danceType: board.dance_type,
              });
              setPreview(null);
              setImportBatch(undefined);
              return true;
            }}
          >
            <ProFormSelect
              name="season_id"
              label="赛季"
              rules={[{ required: true }]}
              request={async () => {
                const list = await listSeasons();
                setSeasons(list.map((s) => ({ label: s.name, value: s.id })));
                return list.map((s) => ({ label: s.name, value: s.id }));
              }}
            />
            <ProFormSelect
              name="event_id"
              label="赛事"
              rules={[{ required: true }]}
              options={eventOptions}
              dependencies={['season_id']}
              request={async () => eventOptions}
            />
            <ProFormSelect
              name="board_index"
              label="榜单"
              rules={[{ required: true }]}
              options={BOARDS.map((b, i) => ({
                label: boardLabel(b.level, b.dance_type),
                value: i,
              }))}
            />
            <Alert
              type="info"
              showIcon
              message="同一场赛事 6 个榜单需分别录入；每天只比一个舞种，实际每场只需 3 个榜单。"
            />
          </StepsForm.StepForm>

          <StepsForm.StepForm
            title="导入 / 录入成绩"
            onFinish={async (values) => {
              if (!selection) return false;
              const rows = (values.rows ?? []) as {
                player_name: string;
                id_card_no?: string;
                award?: string;
                advanced_to_final?: boolean;
                remark?: string;
              }[];
              if (rows.length === 0) {
                if (preview) return true;
                message.warning('请上传成绩 Excel 或至少录入一行成绩');
                return false;
              }
              const result = await previewScores(
                selection.eventId,
                selection.level,
                selection.danceType,
                { rows },
              );
              setPreview(result);
              setImportBatch(undefined);
              return true;
            }}
          >
            <Typography.Title level={5}>上传成绩 Excel（主要方式）</Typography.Title>
            <Typography.Paragraph type="secondary">
              模板列：姓名、证件号、奖项、晋级决赛（可选）、备注（可选）。按证件号唯一匹配选手；
              全部行无错误时，大陆身份证选手会自动建档（其他证件请先在人工报名建档）。
            </Typography.Paragraph>
            <Space style={{ marginBottom: 16 }}>
              <Upload
                accept=".xlsx"
                showUploadList={false}
                beforeUpload={(file) => handleUpload(file)}
              >
                <Button loading={importing}>上传成绩 Excel</Button>
              </Upload>
              <Button
                onClick={() => selection && downloadScoreTemplate(selection.eventId)}
              >
                下载导入模板
              </Button>
            </Space>
            {preview && (
              <Alert
                type={preview.errors > 0 ? 'warning' : 'success'}
                showIcon
                style={{ marginBottom: 16 }}
                title={`已解析 ${preview.total_rows} 行：匹配 ${preview.matched}，新建 ${preview.created}，错误 ${preview.errors}`}
              />
            )}
            <Typography.Title level={5}>在线逐行录入（辅助方式）</Typography.Title>
            <ProFormList
              name="rows"
              copyIconProps={false}
              creatorButtonProps={{ creatorButtonText: '添加一行' }}
            >
              <Space align="baseline" wrap>
                <ProFormText
                  name="player_name"
                  placeholder="选手姓名"
                  rules={[{ required: true }]}
                />
                <ProFormText
                  name="id_card_no"
                  placeholder="证件号（唯一匹配键）"
                />
                <ProFormSelect
                  name="award"
                  placeholder="奖项"
                  options={AWARD_OPTIONS}
                  width="sm"
                />
                <ProFormSwitch name="advanced_to_final" label="晋级决赛" />
                <ProFormText name="remark" placeholder="备注" />
              </Space>
            </ProFormList>
          </StepsForm.StepForm>

          <StepsForm.StepForm title="确认发布">
            {preview && (
              <>
                <Alert
                  type="warning"
                  showIcon
                  style={{ marginBottom: 12 }}
                  title={`共 ${preview.total_rows} 行：匹配成功 ${preview.matched}，新建 ${preview.created}，错误 ${preview.errors}。错误行需修正后重新导入。`}
                />
                <ProTable<ScorePreview['rows'][number]>
                  rowKey="row_no"
                  size="small"
                  search={false}
                  options={false}
                  pagination={false}
                  dataSource={preview.rows}
                  columns={previewColumns}
                />
                <Alert
                  type="info"
                  showIcon
                  style={{ marginTop: 12 }}
                  title="发布后该榜单小程序端即时更新；已发布成绩的修改须走「积分与榜单 → 成绩纠错」流程，全程留痕。"
                />
              </>
            )}
          </StepsForm.StepForm>
        </StepsForm>
      </Card>

      {selection && (
        <Card title="已发布成绩（纠错）" size="small" style={{ marginTop: 16 }}>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            title="修改已发布成绩会把旧记录标记为已纠错并写入新记录，积分与排名自动重算。"
          />
          <ProTable<ScoreEntryRow>
            rowKey="entry_id"
            size="small"
            search={false}
            options={false}
            pagination={false}
            request={async () => {
              const list = await listScoreEntries(
                selection.eventId,
                selection.level,
                selection.danceType,
              );
              return { data: list, success: true };
            }}
            columns={[
              { title: '选手', dataIndex: 'player_name', width: 140 },
              { title: '编号', dataIndex: 'player_no', width: 120 },
              {
                title: '奖项',
                dataIndex: 'award',
                width: 120,
                render: (_, r) =>
                  AWARD_OPTIONS.find((a) => a.value === r.award)?.label ?? '-',
              },
              { title: '积分', dataIndex: 'points', width: 90 },
              {
                title: '晋级决赛',
                dataIndex: 'advanced_to_final',
                width: 100,
                render: (_, r) => (r.advanced_to_final ? '是' : '-'),
              },
              { title: '备注', dataIndex: 'remark' },
              {
                title: '操作',
                valueType: 'option',
                width: 90,
                render: (_, r) => (
                  <Typography.Link
                    onClick={() => {
                      setCorrectTarget(r);
                      correctForm.setFieldsValue({
                        award: r.award ?? undefined,
                        remark: r.remark ?? undefined,
                      });
                    }}
                  >
                    纠错
                  </Typography.Link>
                ),
              },
            ]}
          />
        </Card>
      )}

      <Modal
        title={correctTarget ? `成绩纠错：${correctTarget.player_name}` : ''}
        open={!!correctTarget}
        onCancel={() => setCorrectTarget(null)}
        onOk={() => correctForm.submit()}
      >
        <Form
          form={correctForm}
          layout="vertical"
          onFinish={async (values) => {
            if (!correctTarget) return;
            await correctScore(correctTarget.entry_id, {
              award: values.award,
              remark: values.remark,
            });
            message.success('已纠错并重算积分');
            setCorrectTarget(null);
            return true;
          }}
        >
          <Form.Item name="award" label="奖项">
            <Select allowClear placeholder="保持不变" options={AWARD_OPTIONS} />
          </Form.Item>
          <Form.Item name="remark" label="纠错说明">
            <Input.TextArea rows={3} placeholder="建议填写纠错原因（留痕）" />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  );
};

export default ScoreEntry;
