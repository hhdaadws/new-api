import React, { useEffect, useState, useRef } from 'react';
import { Button, Col, Form, Row, Spin, Typography } from '@douyinfe/semi-ui';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

export default function ServiceTierSettings(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    'service_tier_setting.priority_ratio': 2,
  });
  const refForm = useRef();
  const [inputsRow, setInputsRow] = useState(inputs);

  function handleFieldChange(fieldName) {
    return (value) => {
      setInputs((inputs) => ({ ...inputs, [fieldName]: value }));
    };
  }

  function onSubmit() {
    const updateArray = compareObjects(inputs, inputsRow);
    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));
    const requestQueue = updateArray.map((item) => {
      return API.put('/api/option/', {
        key: item.key,
        value: String(inputs[item.key]),
      });
    });
    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        if (res.includes(undefined))
          return showError(t('部分保存失败，请重试'));
        showSuccess(t('保存成功'));
        props.refresh();
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  }

  useEffect(() => {
    const currentInputs = {};
    for (let key in props.options) {
      if (Object.keys(inputs).includes(key)) {
        currentInputs[key] = props.options[key];
      }
    }
    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    refForm.current.setValues(currentInputs);
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
            {t('当渠道启用 service_tier 透传时，用户请求中携带的 service_tier 将影响计费倍率。需在渠道设置中启用「允许 service_tier 透传」后生效。')}
          </Typography.Text>
          <Row gutter={16}>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field={'service_tier_setting.priority_ratio'}
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
          <Row>
            <Button size='default' onClick={onSubmit}>
              {t('保存 Service Tier 设置')}
            </Button>
          </Row>
        </Form.Section>
      </Form>
    </Spin>
  );
}
