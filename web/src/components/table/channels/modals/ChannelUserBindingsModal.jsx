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

import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Modal,
  Button,
  Table,
  Tag,
  Typography,
  Space,
  Popconfirm,
  Empty,
  Spin,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
} from '../../../../helpers';

const { Text } = Typography;

const ChannelUserBindingsModal = ({ visible, channelId, channelName, onClose }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [bindings, setBindings] = useState([]);
  const [maxUsers, setMaxUsers] = useState(0);
  const [activeCount, setActiveCount] = useState(0);
  const [expireMinutes, setExpireMinutes] = useState(0);

  const loadBindings = useCallback(async () => {
    if (!channelId) return;
    setLoading(true);
    try {
      const res = await API.get(`/api/channel/${channelId}/user_bindings`);
      const { success, data, message } = res.data;
      if (success) {
        setBindings(data.bindings || []);
        setMaxUsers(data.max_users || 0);
        setActiveCount(data.active_count || 0);
        setExpireMinutes(data.expire_minutes || 0);
      } else {
        showError(message);
      }
    } catch (err) {
      showError(err.message);
    }
    setLoading(false);
  }, [channelId]);

  useEffect(() => {
    if (visible && channelId) {
      loadBindings();
    }
  }, [visible, channelId, loadBindings]);

  const handleRelease = async (userId) => {
    try {
      const res = await API.delete(`/api/channel/${channelId}/user_bindings/${userId}`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('释放成功'));
        loadBindings();
      } else {
        showError(message);
      }
    } catch (err) {
      showError(err.message);
    }
  };

  const handleReleaseAll = async () => {
    try {
      const res = await API.delete(`/api/channel/${channelId}/user_bindings`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('全部释放成功'));
        loadBindings();
      } else {
        showError(message);
      }
    } catch (err) {
      showError(err.message);
    }
  };

  const columns = [
    {
      title: t('用户ID'),
      dataIndex: 'user_id',
      width: 80,
    },
    {
      title: t('用户名'),
      dataIndex: 'username',
      width: 120,
      render: (text) => text || '-',
    },
    {
      title: t('绑定时间'),
      dataIndex: 'created_time',
      width: 160,
      render: (value) => (value ? timestamp2string(value) : '-'),
    },
    {
      title: t('最后使用'),
      dataIndex: 'last_used_time',
      width: 160,
      render: (value) => (value ? timestamp2string(value) : '-'),
    },
    {
      title: t('操作'),
      width: 80,
      render: (text, record) => (
        <Popconfirm
          title={t('确定释放该用户的绑定？')}
          onConfirm={() => handleRelease(record.user_id)}
        >
          <Button size='small' type='danger' theme='light'>
            {t('释放')}
          </Button>
        </Popconfirm>
      ),
    },
  ];

  const title = channelName
    ? `${t('用户绑定')} - ${channelName} (#${channelId})`
    : `${t('用户绑定')} (#${channelId})`;

  return (
    <Modal
      title={title}
      visible={visible}
      onCancel={onClose}
      footer={null}
      width={700}
      style={{ maxHeight: '80vh' }}
    >
      <Spin spinning={loading}>
        <Space style={{ marginBottom: 12, width: '100%', justifyContent: 'space-between' }}>
          <Space>
            <Tag color='blue'>
              {t('上限')}: {maxUsers || t('无限制')}
            </Tag>
            <Tag color='green'>
              {t('当前绑定')}: {activeCount}
            </Tag>
            {expireMinutes > 0 && (
              <Tag color='orange'>
                {t('过期时间')}: {expireMinutes} {t('分钟')}
              </Tag>
            )}
            {expireMinutes === 0 && (
              <Tag color='grey'>
                {t('不自动过期')}
              </Tag>
            )}
          </Space>
          {bindings.length > 0 && (
            <Popconfirm
              title={t('确定释放所有用户的绑定？')}
              onConfirm={handleReleaseAll}
            >
              <Button size='small' type='danger'>
                {t('释放全部')}
              </Button>
            </Popconfirm>
          )}
        </Space>

        {bindings.length === 0 && !loading ? (
          <Empty
            image={<IllustrationNoResult />}
            darkModeImage={<IllustrationNoResultDark />}
            description={t('暂无用户绑定')}
            style={{ padding: 40 }}
          />
        ) : (
          <Table
            columns={columns}
            dataSource={bindings}
            rowKey='user_id'
            pagination={false}
            size='small'
          />
        )}
      </Spin>
    </Modal>
  );
};

export default ChannelUserBindingsModal;
