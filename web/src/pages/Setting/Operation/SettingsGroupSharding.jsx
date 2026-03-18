import React, { useState, useEffect, useCallback } from 'react';
import {
  Button,
  Table,
  Tag,
  Space,
  Popconfirm,
  Modal,
  Form,
  InputNumber,
  Select,
  Switch,
  Typography,
  Banner,
} from '@douyinfe/semi-ui';
import { IconPlus, IconRefresh } from '@douyinfe/semi-icons';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

export default function SettingsGroupSharding() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [shards, setShards] = useState([]);
  const [groups, setGroups] = useState([]);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingShard, setEditingShard] = useState(null);
  const [submitting, setSubmitting] = useState(false);

  const loadShards = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/group_shard/');
      if (res.data.success) {
        setShards(res.data.data || []);
      } else {
        showError(res.data.message);
      }
    } catch {
      showError(t('获取分片列表失败'));
    }
    setLoading(false);
  }, [t]);

  const loadGroups = useCallback(async () => {
    try {
      const res = await API.get('/api/group/');
      if (res.data.success) {
        setGroups(res.data.data || []);
      }
    } catch {
      // ignore
    }
  }, []);

  useEffect(() => {
    loadShards();
    loadGroups();
  }, [loadShards, loadGroups]);

  const handleAdd = () => {
    setEditingShard(null);
    setModalVisible(true);
  };

  const handleEdit = (record) => {
    setEditingShard(record);
    setModalVisible(true);
  };

  const handleDelete = async (id) => {
    try {
      const res = await API.delete(`/api/group_shard/${id}`);
      if (res.data.success) {
        showSuccess(t('删除成功'));
        loadShards();
      } else {
        showError(res.data.message);
      }
    } catch {
      showError(t('删除失败'));
    }
  };

  const handleRecount = async () => {
    try {
      const res = await API.post('/api/group_shard/recount');
      if (res.data.success) {
        showSuccess(res.data.message || t('重新统计完成'));
        loadShards();
      } else {
        showError(res.data.message);
      }
    } catch {
      showError(t('重新统计失败'));
    }
  };

  const handleSubmit = async (values) => {
    setSubmitting(true);
    try {
      let res;
      if (editingShard) {
        res = await API.put('/api/group_shard/', {
          ...values,
          id: editingShard.id,
        });
      } else {
        res = await API.post('/api/group_shard/', values);
      }
      if (res.data.success) {
        showSuccess(editingShard ? t('更新成功') : t('创建成功'));
        setModalVisible(false);
        loadShards();
      } else {
        showError(res.data.message);
      }
    } catch {
      showError(editingShard ? t('更新失败') : t('创建失败'));
    }
    setSubmitting(false);
  };

  const columns = [
    {
      title: t('父分组'),
      dataIndex: 'parent_group',
      key: 'parent_group',
      render: (text) => (
        <Tag color='blue' size='small'>
          {text}
        </Tag>
      ),
    },
    {
      title: t('分片名称'),
      dataIndex: 'shard_group',
      key: 'shard_group',
      render: (text) => <Text strong>{text}</Text>,
    },
    {
      title: t('用户数 / 上限'),
      key: 'users',
      render: (_, record) => {
        const current = record.current_users || 0;
        const max = record.max_users || 0;
        const isFull = max > 0 && current >= max;
        return (
          <Space>
            <Text type={isFull ? 'danger' : undefined}>
              {current}
            </Text>
            <Text type='tertiary'>/</Text>
            <Text type='tertiary'>
              {max === 0 ? t('无限制') : max}
            </Text>
          </Space>
        );
      },
    },
    {
      title: t('状态'),
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled) =>
        enabled ? (
          <Tag color='green' size='small'>
            {t('启用')}
          </Tag>
        ) : (
          <Tag color='grey' size='small'>
            {t('禁用')}
          </Tag>
        ),
    },
    {
      title: t('排序'),
      dataIndex: 'sort_order',
      key: 'sort_order',
    },
    {
      title: t('操作'),
      key: 'action',
      fixed: 'right',
      width: 150,
      render: (_, record) => (
        <Space>
          <Button size='small' onClick={() => handleEdit(record)}>
            {t('编辑')}
          </Button>
          <Popconfirm
            title={t('确定删除此分片？')}
            content={
              record.current_users > 0
                ? t('该分片下仍有用户，删除可能导致问题')
                : undefined
            }
            onConfirm={() => handleDelete(record.id)}
          >
            <Button size='small' type='danger'>
              {t('删除')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Banner
        type='info'
        description={t(
          '分组分片功能允许将一个父分组（如 test）拆分为多个子分片（如 test_1, test_2），每个分片有独立的用户上限。用户只看到父分组名，但实际被分配到具体分片。分片用户可以使用父分组的渠道 + 自己分片独有的渠道。'
        )}
        style={{ marginBottom: 16 }}
        closeIcon={null}
      />
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          marginBottom: 16,
        }}
      >
        <Space>
          <Button
            icon={<IconPlus />}
            type='primary'
            theme='solid'
            size='small'
            onClick={handleAdd}
          >
            {t('新增分片')}
          </Button>
          <Button
            icon={<IconRefresh />}
            size='small'
            onClick={handleRecount}
          >
            {t('重新统计用户数')}
          </Button>
        </Space>
      </div>
      <Table
        columns={columns}
        dataSource={shards}
        rowKey='id'
        loading={loading}
        size='small'
        pagination={false}
        scroll={{ x: 'max-content' }}
      />

      <Modal
        title={editingShard ? t('编辑分片') : t('新增分片')}
        visible={modalVisible}
        onCancel={() => setModalVisible(false)}
        footer={null}
        closeOnEsc
      >
        <Form
          onSubmit={handleSubmit}
          initValues={
            editingShard || {
              parent_group: '',
              shard_group: '',
              max_users: 0,
              sort_order: 0,
              enabled: true,
            }
          }
        >
          <Form.Select
            field='parent_group'
            label={t('父分组')}
            rules={[{ required: true, message: t('请选择父分组') }]}
            disabled={!!editingShard}
          >
            {groups.map((g) => (
              <Select.Option key={g} value={g}>
                {g}
              </Select.Option>
            ))}
          </Form.Select>
          <Form.Input
            field='shard_group'
            label={t('分片名称')}
            placeholder={t('例如: test_1')}
            rules={[{ required: true, message: t('请输入分片名称') }]}
            disabled={!!editingShard}
          />
          <Form.InputNumber
            field='max_users'
            label={t('最大用户数')}
            extraText={t('0 表示无限制')}
            min={0}
          />
          <Form.InputNumber
            field='sort_order'
            label={t('排序')}
            extraText={t('数字越小优先级越高')}
            min={0}
          />
          <Form.Switch
            field='enabled'
            label={t('启用')}
          />
          <div style={{ marginTop: 16, textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setModalVisible(false)}>
                {t('取消')}
              </Button>
              <Button
                type='primary'
                theme='solid'
                htmlType='submit'
                loading={submitting}
              >
                {editingShard ? t('更新') : t('创建')}
              </Button>
            </Space>
          </div>
        </Form>
      </Modal>
    </div>
  );
}
