/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useState, useRef } from 'react';
import {
  Button,
  Col,
  Form,
  Row,
  Spin,
  InputNumber,
  Typography,
  Space,
} from '@douyinfe/semi-ui';
import { IconPlus, IconDelete } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';

export default function SettingsCreditLimit(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    QuotaForNewUser: '',
    PreConsumedQuota: '',
    QuotaForInviter: '',
    QuotaForInvitee: '',
    'quota_setting.enable_free_model_pre_consume': true,
    InviterCommissionRates: '',
  });
  const refForm = useRef();
  const [inputsRow, setInputsRow] = useState(inputs);
  const [commissionRules, setCommissionRules] = useState([]);

  function parseCommissionRates(jsonStr) {
    if (!jsonStr || jsonStr === '{}') return [];
    try {
      const obj = JSON.parse(jsonStr);
      return Object.entries(obj)
        .map(([k, v]) => ({ orderNumber: parseInt(k, 10), rate: v }))
        .sort((a, b) => a.orderNumber - b.orderNumber);
    } catch {
      return [];
    }
  }

  function serializeCommissionRules(rules) {
    const obj = {};
    rules.forEach((r) => {
      if (r.orderNumber > 0 && r.rate >= 0) {
        obj[r.orderNumber] = r.rate;
      }
    });
    return JSON.stringify(obj);
  }

  function addCommissionRule() {
    const maxOrder =
      commissionRules.length > 0
        ? Math.max(...commissionRules.map((r) => r.orderNumber))
        : 0;
    const newRules = [
      ...commissionRules,
      { orderNumber: maxOrder + 1, rate: 10 },
    ];
    setCommissionRules(newRules);
    const serialized = serializeCommissionRules(newRules);
    setInputs({ ...inputs, InviterCommissionRates: serialized });
  }

  function removeCommissionRule(index) {
    const newRules = commissionRules.filter((_, i) => i !== index);
    setCommissionRules(newRules);
    const serialized = serializeCommissionRules(newRules);
    setInputs({ ...inputs, InviterCommissionRates: serialized });
  }

  function updateCommissionRule(index, field, value) {
    const newRules = [...commissionRules];
    newRules[index] = { ...newRules[index], [field]: value };
    setCommissionRules(newRules);
    const serialized = serializeCommissionRules(newRules);
    setInputs({ ...inputs, InviterCommissionRates: serialized });
  }

  function onSubmit() {
    const updateArray = compareObjects(inputs, inputsRow);
    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));
    const requestQueue = updateArray.map((item) => {
      let value = '';
      if (typeof inputs[item.key] === 'boolean') {
        value = String(inputs[item.key]);
      } else {
        value = inputs[item.key];
      }
      return API.put('/api/option/', {
        key: item.key,
        value,
      });
    });
    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        if (requestQueue.length === 1) {
          if (res.includes(undefined)) return;
        } else if (requestQueue.length > 1) {
          if (res.includes(undefined))
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
    setCommissionRules(
      parseCommissionRates(currentInputs.InviterCommissionRates),
    );
  }, [props.options]);
  return (
    <>
      <Spin spinning={loading}>
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('额度设置')}>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('新用户初始额度')}
                  field={'QuotaForNewUser'}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  placeholder={''}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      QuotaForNewUser: String(value),
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('请求预扣费额度')}
                  field={'PreConsumedQuota'}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  extraText={t('请求结束后多退少补')}
                  placeholder={''}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      PreConsumedQuota: String(value),
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('邀请新用户奖励额度')}
                  field={'QuotaForInviter'}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  extraText={''}
                  placeholder={t('例如：2000')}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      QuotaForInviter: String(value),
                    })
                  }
                />
              </Col>
            </Row>
            <Row>
              <Col xs={24} sm={12} md={8} lg={8} xl={6}>
                <Form.InputNumber
                  label={t('新用户使用邀请码奖励额度')}
                  field={'QuotaForInvitee'}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  extraText={''}
                  placeholder={t('例如：1000')}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      QuotaForInvitee: String(value),
                    })
                  }
                />
              </Col>
            </Row>
            <Row>
              <Col>
                <Form.Switch
                  label={t('对免费模型启用预消耗')}
                  field={'quota_setting.enable_free_model_pre_consume'}
                  extraText={t(
                    '开启后，对免费模型（倍率为0，或者价格为0）的模型也会预消耗额度',
                  )}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'quota_setting.enable_free_model_pre_consume': value,
                    })
                  }
                />
              </Col>
            </Row>
          </Form.Section>

          <Form.Section text={t('充值返佣设置')}>
            <Typography.Text type='tertiary' style={{ marginBottom: 12, display: 'block' }}>
              {t('为被邀请用户的每笔充值设置返佣比例，邀请人将按比例获得额度奖励')}
            </Typography.Text>
            {commissionRules.map((rule, index) => (
              <Row
                key={index}
                gutter={16}
                style={{ marginBottom: 8 }}
                type='flex'
                align='middle'
              >
                <Col>
                  <Space>
                    <Typography.Text>{t('第')}</Typography.Text>
                    <InputNumber
                      min={1}
                      step={1}
                      value={rule.orderNumber}
                      style={{ width: 80 }}
                      onChange={(value) =>
                        updateCommissionRule(index, 'orderNumber', value)
                      }
                    />
                    <Typography.Text>{t('笔订单')}</Typography.Text>
                    <Typography.Text>{t('返佣比例')}</Typography.Text>
                    <InputNumber
                      min={0}
                      max={100}
                      step={1}
                      value={rule.rate}
                      style={{ width: 100 }}
                      suffix='%'
                      onChange={(value) =>
                        updateCommissionRule(index, 'rate', value)
                      }
                    />
                    <Button
                      type='danger'
                      theme='borderless'
                      icon={<IconDelete />}
                      onClick={() => removeCommissionRule(index)}
                    />
                  </Space>
                </Col>
              </Row>
            ))}
            <Row style={{ marginTop: 8 }}>
              <Button
                icon={<IconPlus />}
                theme='light'
                onClick={addCommissionRule}
              >
                {t('添加返佣规则')}
              </Button>
            </Row>
          </Form.Section>

          <Row style={{ marginTop: 16 }}>
            <Button size='default' onClick={onSubmit}>
              {t('保存额度设置')}
            </Button>
          </Row>
        </Form>
      </Spin>
    </>
  );
}
