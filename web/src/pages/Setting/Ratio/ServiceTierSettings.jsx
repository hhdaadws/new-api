import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Button,
  Col,
  Form,
  Input,
  InputNumber,
  Row,
  Space,
  Spin,
  Table,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import {
  API,
  compareObjects,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

const DEFAULT_INPUTS = {
  'service_tier_setting.priority_ratio': 2,
  'service_tier_setting.priority_model_ratios': '{}',
};

const normalizeModelRatios = (value) => {
  if (!value || String(value).trim() === '') {
    return {};
  }
  const parsed = typeof value === 'string' ? JSON.parse(value) : value;
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    throw new Error('JSON 必须是对象');
  }
  return Object.entries(parsed).reduce((acc, [model, ratio]) => {
    const key = String(model).trim();
    const numberRatio = Number(ratio);
    if (!key) {
      throw new Error('模型名不能为空');
    }
    if (!Number.isFinite(numberRatio) || numberRatio <= 0) {
      throw new Error('倍率必须大于 0');
    }
    acc[key] = numberRatio;
    return acc;
  }, {});
};

const rowsToRatios = (rows) => {
  return rows.reduce((acc, row) => {
    const model = String(row.model || '').trim();
    const ratio = Number(row.ratio);
    if (!model) {
      throw new Error('模型名不能为空');
    }
    if (!Number.isFinite(ratio) || ratio <= 0) {
      throw new Error('倍率必须大于 0');
    }
    acc[model] = ratio;
    return acc;
  }, {});
};

const ratiosToRows = (ratios) => {
  return Object.entries(ratios).map(([model, ratio], index) => ({
    id: `${model}-${index}`,
    model,
    ratio,
  }));
};

export default function ServiceTierSettings(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState(DEFAULT_INPUTS);
  const [inputsRow, setInputsRow] = useState(DEFAULT_INPUTS);
  const [rows, setRows] = useState([]);
  const [jsonText, setJsonText] = useState('{}');
  const refForm = useRef();

  const updateRowsAndJson = (nextRows) => {
    setRows(nextRows);
    try {
      const ratios = rowsToRatios(nextRows);
      const serialized = JSON.stringify(ratios, null, 2);
      setJsonText(serialized);
      setInputs((prev) => ({
        ...prev,
        'service_tier_setting.priority_model_ratios': serialized,
      }));
    } catch (error) {
      setRows(nextRows);
    }
  };

  const columns = useMemo(
    () => [
      {
        title: t('模型或前缀'),
        dataIndex: 'model',
        render: (_, record, index) => (
          <Input
            value={record.model}
            placeholder={t('gpt-5.4*')}
            onChange={(value) => {
              const nextRows = rows.map((row, rowIndex) =>
                rowIndex === index ? { ...row, model: value } : row,
              );
              updateRowsAndJson(nextRows);
            }}
          />
        ),
      },
      {
        title: t('倍率'),
        dataIndex: 'ratio',
        width: 180,
        render: (_, record, index) => (
          <InputNumber
            value={record.ratio}
            min={0.1}
            step={0.1}
            style={{ width: '100%' }}
            onChange={(value) => {
              const nextRows = rows.map((row, rowIndex) =>
                rowIndex === index ? { ...row, ratio: value } : row,
              );
              updateRowsAndJson(nextRows);
            }}
          />
        ),
      },
      {
        title: '',
        dataIndex: 'operate',
        width: 110,
        render: (_, record) => (
          <Button
            type='danger'
            theme='borderless'
            onClick={() => updateRowsAndJson(rows.filter((row) => row !== record))}
          >
            {t('删除')}
          </Button>
        ),
      },
    ],
    [rows, t],
  );

  const handleFieldChange = (fieldName) => (value) => {
    setInputs((prev) => ({ ...prev, [fieldName]: value }));
  };

  const handleJsonChange = (value) => {
    setJsonText(value);
    try {
      const ratios = normalizeModelRatios(value);
      const serialized = JSON.stringify(ratios, null, 2);
      setRows(ratiosToRows(ratios));
      setInputs((prev) => ({
        ...prev,
        'service_tier_setting.priority_model_ratios': serialized,
      }));
    } catch (error) {
      setInputs((prev) => ({
        ...prev,
        'service_tier_setting.priority_model_ratios': value,
      }));
    }
  };

  const addRow = () => {
    updateRowsAndJson([
      ...rows,
      {
        id: `new-${Date.now()}`,
        model: '',
        ratio: Number(inputs['service_tier_setting.priority_ratio']) || 2,
      },
    ]);
  };

  const onSubmit = () => {
    let modelRatios;
    try {
      modelRatios = rowsToRatios(rows);
      normalizeModelRatios(jsonText);
    } catch (error) {
      showError(t(error.message));
      return;
    }

    const priorityRatio = Number(inputs['service_tier_setting.priority_ratio']);
    if (!Number.isFinite(priorityRatio) || priorityRatio <= 0) {
      showError(t('倍率必须大于 0'));
      return;
    }

    const payload = {
      'service_tier_setting.priority_ratio': String(priorityRatio),
      'service_tier_setting.priority_model_ratios': JSON.stringify(
        modelRatios,
        null,
        2,
      ),
    };
    const updateArray = compareObjects(payload, inputsRow);
    if (!updateArray.length) {
      return showWarning(t('你似乎并没有修改什么'));
    }

    setLoading(true);
    Promise.all(
      updateArray.map((item) =>
        API.put('/api/option/', {
          key: item.key,
          value: payload[item.key],
        }),
      ),
    )
      .then((res) => {
        if (res.includes(undefined)) {
          return showError(t('部分保存失败，请重试'));
        }
        showSuccess(t('保存成功'));
        props.refresh();
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  };

  useEffect(() => {
    const currentInputs = { ...DEFAULT_INPUTS };
    Object.keys(DEFAULT_INPUTS).forEach((key) => {
      if (props.options[key] !== undefined) {
        currentInputs[key] = props.options[key];
      }
    });

    let ratios = {};
    try {
      ratios = normalizeModelRatios(
        currentInputs['service_tier_setting.priority_model_ratios'],
      );
    } catch (error) {
      ratios = {};
    }
    const serialized = JSON.stringify(ratios, null, 2);
    currentInputs['service_tier_setting.priority_model_ratios'] = serialized;

    setInputs(currentInputs);
    setInputsRow({ ...currentInputs });
    setRows(ratiosToRows(ratios));
    setJsonText(serialized);
    refForm.current?.setValues(currentInputs);
  }, [props.options]);

  return (
    <Spin spinning={loading}>
      <Form
        values={inputs}
        getFormApi={(formAPI) => (refForm.current = formAPI)}
        style={{ marginBottom: 15 }}
      >
        <Form.Section text={t('Service Tier 计费设置')}>
          <Typography.Text
            type='tertiary'
            style={{ marginBottom: 16, display: 'block' }}
          >
            {t(
              '当渠道启用 service_tier 透传时，用户请求中携带的 service_tier 将影响计费倍率。需在渠道设置中启用「允许 service_tier 透传」后生效。',
            )}
          </Typography.Text>
          <Row gutter={16}>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='service_tier_setting.priority_ratio'
                label={t('Priority 层级倍率')}
                placeholder='2'
                step={0.1}
                min={0.1}
                onChange={handleFieldChange(
                  'service_tier_setting.priority_ratio',
                )}
              />
            </Col>
          </Row>
          <Space vertical align='start' style={{ width: '100%' }}>
            <Typography.Text strong>{t('分模型倍率')}</Typography.Text>
            <Table
              columns={columns}
              dataSource={rows}
              pagination={false}
              rowKey='id'
              size='small'
              style={{ width: '100%' }}
              empty={t('暂无数据')}
            />
            <Button onClick={addRow}>{t('新增模型倍率')}</Button>
            <TextArea
              value={jsonText}
              autosize={{ minRows: 6, maxRows: 12 }}
              onChange={handleJsonChange}
              placeholder={t('{"gpt-5.4": 2, "gpt-5.5": 2.5, "gpt-5.4*": 2}')}
              style={{ width: '100%' }}
            />
            <Typography.Text type='tertiary'>
              {t(
                '精确模型名优先，带 * 的配置按最长前缀匹配；未命中时使用全局倍率。',
              )}
            </Typography.Text>
            <Button type='primary' onClick={onSubmit}>
              {t('保存 Service Tier 设置')}
            </Button>
          </Space>
        </Form.Section>
      </Form>
    </Spin>
  );
}
