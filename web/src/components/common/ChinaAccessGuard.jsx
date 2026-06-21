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

import React, { useContext, useState } from 'react';
import { Modal, Button, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { StatusContext } from '../../context/Status';

const CONSENT_KEY = 'china_access_consent';

/**
 * ChinaAccessGuard 中国大陆访问限制守卫。
 *
 * 根据 /api/status 返回的 china_block_mode 与 is_china_mainland_ip:
 *   - off   : 不做任何处理
 *   - popup : 命中大陆 IP 时弹窗提示,用户必须点击"同意"后方可继续使用前端
 *   - block : 命中大陆 IP 时全屏拦截(服务端通常已拦截,这里作为前端兜底)
 *
 * 仅作用于前端页面,不影响任何 API 请求。
 */
const ChinaAccessGuard = () => {
  const { t } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const status = statusState?.status;

  const [consented, setConsented] = useState(
    () => localStorage.getItem(CONSENT_KEY) === 'true',
  );

  const mode = status?.china_block_mode;
  const isCN = status?.is_china_mainland_ip;
  const title = status?.china_block_title || t('访问地区提示');
  const content = status?.china_block_content || '';

  // 未命中大陆 IP 或未开启限制时不处理
  if (!isCN || !mode || mode === 'off') {
    return null;
  }

  const handleAgree = () => {
    localStorage.setItem(CONSENT_KEY, 'true');
    setConsented(true);
  };

  // 拦截模式:全屏遮罩,无法继续使用
  if (mode === 'block') {
    return (
      <div
        style={{
          position: 'fixed',
          inset: 0,
          zIndex: 100000,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: '#0f172a',
          color: '#e2e8f0',
          padding: 24,
        }}
      >
        <div style={{ maxWidth: 560, textAlign: 'center', lineHeight: 1.8 }}>
          <Typography.Title heading={3} style={{ color: '#e2e8f0' }}>
            {title}
          </Typography.Title>
          <Typography.Paragraph
            style={{ color: '#94a3b8', marginTop: 16, whiteSpace: 'pre-wrap' }}
          >
            {content}
          </Typography.Paragraph>
        </div>
      </div>
    );
  }

  // 弹窗模式:已同意则不再提示
  if (mode === 'popup' && consented) {
    return null;
  }

  return (
    <Modal
      title={title}
      visible={true}
      closable={false}
      maskClosable={false}
      keepDOMMounted={false}
      footer={
        <Button theme='solid' type='primary' onClick={handleAgree}>
          {t('我已知晓并同意')}
        </Button>
      }
    >
      <Typography.Paragraph style={{ whiteSpace: 'pre-wrap', lineHeight: 1.8 }}>
        {content}
      </Typography.Paragraph>
    </Modal>
  );
};

export default ChinaAccessGuard;
