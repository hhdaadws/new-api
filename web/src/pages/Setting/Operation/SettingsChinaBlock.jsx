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
import { Button, Col, Form, Row, Spin, Typography } from '@douyinfe/semi-ui';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

export default function SettingsChinaBlock(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    'china_block.mode': 'off',
    'china_block.title': '',
    'china_block.content': '',
    'china_block.whitelist': '',
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
      const value = String(inputs[item.key]);
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
  }, [props.options]);

  return (
    <>
      <Spin spinning={loading}>
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('访问地区限制设置')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t(
                '检测到来自中国大陆的访问时,可选择弹窗提示(同意后继续)或直接拦截前端页面。该设置仅作用于前端页面,不影响任何 API 请求。',
              )}
            </Typography.Text>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Select
                  field={'china_block.mode'}
                  label={t('限制模式')}
                  style={{ width: '100%' }}
                  optionList={[
                    { label: t('关闭限制'), value: 'off' },
                    { label: t('弹窗提示(需同意)'), value: 'popup' },
                    { label: t('直接拦截'), value: 'block' },
                  ]}
                  onChange={handleFieldChange('china_block.mode')}
                />
              </Col>
              <Col xs={24} sm={12} md={16} lg={16} xl={16}>
                <Form.Input
                  field={'china_block.title'}
                  label={t('弹窗/拦截页标题')}
                  placeholder={t('访问地区提示')}
                  onChange={handleFieldChange('china_block.title')}
                  disabled={inputs['china_block.mode'] === 'off'}
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24}>
                <Form.TextArea
                  field={'china_block.content'}
                  label={t('弹窗/拦截页正文')}
                  placeholder={t('请输入提示给用户的正文内容')}
                  autosize={{ minRows: 4, maxRows: 10 }}
                  onChange={handleFieldChange('china_block.content')}
                  disabled={inputs['china_block.mode'] === 'off'}
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24}>
                <Form.TextArea
                  field={'china_block.whitelist'}
                  label={t('IP 白名单(逗号分隔，支持 CIDR)')}
                  placeholder={t('命中白名单的 IP 即使来自大陆也会放行，例如：1.2.3.4, 5.6.7.0/24')}
                  autosize={{ minRows: 2, maxRows: 6 }}
                  onChange={handleFieldChange('china_block.whitelist')}
                  disabled={inputs['china_block.mode'] === 'off'}
                />
              </Col>
            </Row>
            <Row>
              <Button size='default' onClick={onSubmit}>
                {t('保存访问地区限制设置')}
              </Button>
            </Row>
          </Form.Section>
        </Form>
      </Spin>
    </>
  );
}
