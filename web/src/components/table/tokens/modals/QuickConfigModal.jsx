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
import React, { useState, useEffect, useMemo } from 'react';
import {
  Modal,
  RadioGroup,
  Radio,
  Select,
  Button,
  Typography,
  Toast,
} from '@douyinfe/semi-ui';
import { IconCopy } from '@douyinfe/semi-icons';
import { copy, showSuccess, showError } from '../../../../helpers';
import { getServerAddress } from '../../../../helpers/token';
import { selectFilter } from '../../../../helpers';

function detectOS() {
  const ua = navigator.userAgent.toLowerCase();
  if (ua.includes('win')) return 'windows';
  if (ua.includes('mac')) return 'mac';
  return 'linux';
}

function getUrlOptions(t) {
  const serverAddress = getServerAddress();
  const options = [
    {
      label: t('默认服务器地址') + ` (${serverAddress})`,
      value: serverAddress,
    },
  ];

  try {
    const raw = localStorage.getItem('status');
    if (raw) {
      const status = JSON.parse(raw);
      if (Array.isArray(status.api_info)) {
        status.api_info.forEach((info) => {
          options.push({
            label: `${info.route} - ${info.url}`,
            value: info.url,
          });
        });
      }
    }
  } catch (_) {}

  return options;
}

function generateClaudeCommands(os, tokenKey, url) {
  const fullToken = `sk-${tokenKey}`;

  if (os === 'windows') {
    return [
      `setx ANTHROPIC_AUTH_TOKEN "${fullToken}"`,
      `setx ANTHROPIC_BASE_URL "${url}"`,
    ].join('\n');
  }

  // Mac and Linux both use ~/.zshrc
  return [
    `# Claude Code 配置`,
    `grep -q 'ANTHROPIC_AUTH_TOKEN' ~/.zshrc 2>/dev/null && sed -i${os === 'mac' ? " ''" : ''} 's|^export ANTHROPIC_AUTH_TOKEN=.*|export ANTHROPIC_AUTH_TOKEN="${fullToken}"|' ~/.zshrc || echo 'export ANTHROPIC_AUTH_TOKEN="${fullToken}"' >> ~/.zshrc`,
    `grep -q 'ANTHROPIC_BASE_URL' ~/.zshrc 2>/dev/null && sed -i${os === 'mac' ? " ''" : ''} 's|^export ANTHROPIC_BASE_URL=.*|export ANTHROPIC_BASE_URL="${url}"|' ~/.zshrc || echo 'export ANTHROPIC_BASE_URL="${url}"' >> ~/.zshrc`,
    `source ~/.zshrc`,
  ].join('\n');
}

function generateCodexCommands(os, tokenKey, url) {
  const fullToken = `sk-${tokenKey}`;
  const baseUrl = url.endsWith('/') ? url + 'v1' : url + '/v1';

  const configToml = `model_provider = "MikuCode"
model = "gpt-5.4"
model_reasoning_effort = "high"
disable_response_storage = true
preferred_auth_method = "apikey"
[model_providers.MikuCode]
name = "MikuCode"
base_url = "${baseUrl}"
wire_api = "responses"
requires_openai_auth = true`;

  const authJson = `{
  "OPENAI_API_KEY": "${fullToken}"
}`;

  if (os === 'windows') {
    return `New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\\.codex" | Out-Null

@"
${configToml}
"@ | Set-Content -Path "$env:USERPROFILE\\.codex\\config.toml" -Encoding UTF8

@"
${authJson}
"@ | Set-Content -Path "$env:USERPROFILE\\.codex\\auth.json" -Encoding UTF8`;
  }

  return `mkdir -p ~/.codex

cat > ~/.codex/config.toml << 'EOF'
${configToml}
EOF

cat > ~/.codex/auth.json << 'EOF'
${authJson}
EOF`;
}

export default function QuickConfigModal({
  visible,
  onClose,
  tokens,
  fetchTokenKey,
  t,
}) {
  const [selectedTokenId, setSelectedTokenId] = useState(null);
  const [platform, setPlatform] = useState('claude-code');
  const [selectedUrl, setSelectedUrl] = useState('');
  const [os, setOs] = useState('windows');
  const [generatedCommands, setGeneratedCommands] = useState('');
  const [generating, setGenerating] = useState(false);

  const urlOptions = useMemo(() => getUrlOptions(t), [visible, t]);

  const tokenOptions = useMemo(() => {
    return (tokens || [])
      .filter((token) => token.status === 1)
      .map((token) => ({
        label: token.name,
        value: token.id,
      }));
  }, [tokens]);

  useEffect(() => {
    if (visible) {
      setOs(detectOS());
      setGeneratedCommands('');
      setSelectedTokenId(null);
      setPlatform('claude-code');
      setSelectedUrl(getServerAddress());
    }
  }, [visible]);

  const handleGenerate = async () => {
    if (!selectedTokenId) {
      Toast.warning(t('请选择令牌'));
      return;
    }
    if (!selectedUrl) {
      Toast.warning(t('请选择地址'));
      return;
    }

    setGenerating(true);
    try {
      const tokenKey = await fetchTokenKey({ id: selectedTokenId });
      if (!tokenKey) {
        showError(t('获取令牌密钥失败'));
        return;
      }

      let commands = '';
      if (platform === 'claude-code') {
        commands = generateClaudeCommands(os, tokenKey, selectedUrl);
      } else {
        commands = generateCodexCommands(os, tokenKey, selectedUrl);
      }
      setGeneratedCommands(commands);
    } catch (e) {
      showError(e.message || t('获取令牌密钥失败'));
    } finally {
      setGenerating(false);
    }
  };

  const handleCopy = async () => {
    await copy(generatedCommands);
    showSuccess(t('配置命令已复制到剪贴板'));
  };

  const fieldLabelStyle = useMemo(
    () => ({
      marginBottom: 4,
      fontSize: 13,
      color: 'var(--semi-color-text-1)',
    }),
    [],
  );

  return (
    <Modal
      title={t('一键配置')}
      visible={visible}
      onCancel={onClose}
      footer={null}
      maskClosable={false}
      width={640}
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div>
          <div style={fieldLabelStyle}>
            {t('选择令牌')}{' '}
            <Typography.Text type='danger'>*</Typography.Text>
          </div>
          <Select
            placeholder={t('请选择令牌')}
            optionList={tokenOptions}
            value={selectedTokenId}
            onChange={(val) => {
              setSelectedTokenId(val);
              setGeneratedCommands('');
            }}
            filter={selectFilter}
            style={{ width: '100%' }}
            showClear
            searchable
            emptyContent={t('暂无数据')}
          />
        </div>

        <div>
          <div style={fieldLabelStyle}>
            {t('选择平台')}{' '}
            <Typography.Text type='danger'>*</Typography.Text>
          </div>
          <RadioGroup
            type='button'
            value={platform}
            onChange={(e) => {
              setPlatform(e.target.value);
              setGeneratedCommands('');
            }}
            style={{ width: '100%' }}
          >
            <Radio value='claude-code'>Claude Code</Radio>
            <Radio value='codex'>Codex</Radio>
          </RadioGroup>
        </div>

        <div>
          <div style={fieldLabelStyle}>
            {t('选择地址')}{' '}
            <Typography.Text type='danger'>*</Typography.Text>
          </div>
          <Select
            placeholder={t('请选择地址')}
            optionList={urlOptions}
            value={selectedUrl}
            onChange={(val) => {
              setSelectedUrl(val);
              setGeneratedCommands('');
            }}
            filter={selectFilter}
            style={{ width: '100%' }}
            showClear
            searchable
            emptyContent={t('暂无数据')}
          />
        </div>

        <div>
          <div style={fieldLabelStyle}>{t('选择操作系统')}</div>
          <RadioGroup
            type='button'
            value={os}
            onChange={(e) => {
              setOs(e.target.value);
              setGeneratedCommands('');
            }}
            style={{ width: '100%' }}
          >
            <Radio value='windows'>Windows</Radio>
            <Radio value='linux'>Linux</Radio>
            <Radio value='mac'>Mac</Radio>
          </RadioGroup>
        </div>

        <Button
          type='primary'
          theme='solid'
          loading={generating}
          onClick={handleGenerate}
          disabled={!selectedTokenId || !selectedUrl}
        >
          {t('生成配置命令')}
        </Button>

        {generatedCommands && (
          <div style={{ position: 'relative' }}>
            <pre
              style={{
                background: 'var(--semi-color-fill-0)',
                padding: '12px 40px 12px 12px',
                borderRadius: 6,
                overflow: 'auto',
                maxHeight: 350,
                fontSize: 13,
                lineHeight: 1.5,
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-all',
              }}
            >
              <code>{generatedCommands}</code>
            </pre>
            <Button
              size='small'
              icon={<IconCopy />}
              style={{ position: 'absolute', top: 8, right: 8 }}
              onClick={handleCopy}
            />
          </div>
        )}
      </div>
    </Modal>
  );
}
