import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Table,
  Tag,
  Space,
  InputNumber,
  Select,
  Empty,
  Spin,
  Checkbox,
  Typography,
} from '@douyinfe/semi-ui';
import { IconSearch, IconLink, IconRefresh } from '@douyinfe/semi-icons';
import {
  API,
  showError,
  showSuccess,
  renderQuota,
} from '../../../../helpers';
import {
  displayAmountToQuota,
} from '../../../../helpers/quota';

const { Text } = Typography;

const UserProfileAnalysis = ({ groups, maxUsers, channelId }) => {
  const { t } = useTranslation();
  const [selectedGroup, setSelectedGroup] = useState(
    (groups && groups.length > 0) ? groups[0] : 'default'
  );
  const [budgetAmount, setBudgetAmount] = useState('');
  const [analyzing, setAnalyzing] = useState(false);
  const [results, setResults] = useState([]);
  const [selectedUserIds, setSelectedUserIds] = useState([]);
  const [expireMinutes, setExpireMinutes] = useState(0);
  const [binding, setBinding] = useState(false);
  const [hasAnalyzed, setHasAnalyzed] = useState(false);
  const [excludedUserIds, setExcludedUserIds] = useState([]);

  const handleAnalyze = async (overrideExcluded) => {
    if (!budgetAmount || budgetAmount <= 0) {
      showError(t('请输入每周预算'));
      return;
    }
    if (!maxUsers || maxUsers <= 0) {
      showError(t('请先设置最大用户数'));
      return;
    }

    const weeklyBudgetQuota = displayAmountToQuota(budgetAmount);
    if (weeklyBudgetQuota <= 0) {
      showError(t('请输入每周预算'));
      return;
    }

    const excludeIds = overrideExcluded ?? excludedUserIds;

    setAnalyzing(true);
    setResults([]);
    setSelectedUserIds([]);
    setHasAnalyzed(false);
    try {
      const res = await API.post('/api/channel/analyze_users', {
        group: selectedGroup,
        max_count: maxUsers,
        weekly_budget_quota: weeklyBudgetQuota,
        exclude_user_ids: excludeIds.length > 0 ? excludeIds : undefined,
      });
      const { success, data, message } = res.data;
      if (success) {
        const list = data || [];
        setResults(list);
        setSelectedUserIds(list.map((u) => u.user_id));
      } else {
        showError(message);
      }
    } catch (err) {
      showError(err.message);
    }
    setAnalyzing(false);
    setHasAnalyzed(true);
  };

  const handleBind = async () => {
    if (!channelId || selectedUserIds.length === 0) return;
    setBinding(true);
    try {
      const res = await API.post(
        `/api/channel/${channelId}/user_bindings/batch`,
        {
          user_ids: selectedUserIds,
          expire_minutes: expireMinutes || 0,
        }
      );
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('绑定成功'));
        setResults([]);
        setSelectedUserIds([]);
      } else {
        showError(message);
      }
    } catch (err) {
      showError(err.message);
    }
    setBinding(false);
  };

  const handleNextBatch = () => {
    const currentIds = results.map((u) => u.user_id);
    const newExcluded = [...new Set([...excludedUserIds, ...currentIds])];
    setExcludedUserIds(newExcluded);
    handleAnalyze(newExcluded);
  };

  const handleExcludeUser = (userId) => {
    setExcludedUserIds((prev) => [...new Set([...prev, userId])]);
    setResults((prev) => prev.filter((u) => u.user_id !== userId));
    setSelectedUserIds((prev) => prev.filter((id) => id !== userId));
  };

  const handleResetExcluded = () => {
    setExcludedUserIds([]);
  };

  const toggleUser = (userId, checked) => {
    if (checked) {
      setSelectedUserIds((prev) => [...prev, userId]);
    } else {
      setSelectedUserIds((prev) => prev.filter((id) => id !== userId));
    }
  };

  const allSelected =
    results.length > 0 &&
    selectedUserIds.length === results.length;

  const toggleAll = (checked) => {
    if (checked) {
      setSelectedUserIds(results.map((u) => u.user_id));
    } else {
      setSelectedUserIds([]);
    }
  };

  const groupOptions = (groups || ['default']).map((g) => ({
    label: g,
    value: g,
  }));

  const columns = [
    {
      title: (
        <Checkbox
          checked={allSelected}
          indeterminate={
            selectedUserIds.length > 0 &&
            selectedUserIds.length < results.length
          }
          onChange={(e) => toggleAll(e.target.checked)}
        />
      ),
      dataIndex: 'user_id',
      width: 40,
      render: (userId) => (
        <Checkbox
          checked={selectedUserIds.includes(userId)}
          onChange={(e) => toggleUser(userId, e.target.checked)}
        />
      ),
    },
    {
      title: t('用户ID'),
      dataIndex: 'user_id',
      width: 70,
    },
    {
      title: t('用户名'),
      dataIndex: 'username',
      width: 110,
      render: (text) => text || '-',
    },
    {
      title: t('周消费'),
      dataIndex: 'weekly_quota',
      width: 110,
      render: (quota) => renderQuota(quota, 2),
      sorter: (a, b) => a.weekly_quota - b.weekly_quota,
    },
    {
      title: t('请求次数'),
      dataIndex: 'request_count',
      width: 80,
    },
    {
      title: t('常用模型'),
      dataIndex: 'top_models',
      render: (models) => (
        <Space wrap>
          {(models || []).map((m) => (
            <Tag size='small' key={m}>
              {m}
            </Tag>
          ))}
        </Space>
      ),
    },
    {
      title: t('操作'),
      width: 60,
      render: (text, record) => (
        <Button
          size='small'
          type='danger'
          theme='light'
          onClick={() => handleExcludeUser(record.user_id)}
        >
          {t('排除')}
        </Button>
      ),
    },
  ];

  const totalSelectedQuota = results
    .filter((u) => selectedUserIds.includes(u.user_id))
    .reduce((sum, u) => sum + u.weekly_quota, 0);

  return (
    <div
      style={{
        border: '1px solid var(--semi-color-border)',
        borderRadius: 8,
        padding: 16,
        marginTop: 12,
        marginBottom: 12,
      }}
    >
        <Text strong style={{ display: 'block', marginBottom: 12 }}>
          {t('用户画像分析')}
        </Text>

        <Space
          style={{ width: '100%', marginBottom: 12 }}
          align='end'
        >
          <div>
            <Text size='small' style={{ display: 'block', marginBottom: 4 }}>
              {t('分析分组')}
            </Text>
            <Select
              value={selectedGroup}
              onChange={setSelectedGroup}
              optionList={groupOptions}
              style={{ width: 140 }}
              size='small'
            />
          </div>
          <div>
            <Text size='small' style={{ display: 'block', marginBottom: 4 }}>
              {t('每周预算')}
            </Text>
            <InputNumber
              value={budgetAmount}
              onChange={setBudgetAmount}
              min={0}
              style={{ width: 140 }}
              size='small'
              prefix='$'
            />
          </div>
          <Button
            icon={<IconSearch />}
            size='small'
            theme='solid'
            onClick={() => handleAnalyze()}
            loading={analyzing}
            disabled={!maxUsers || maxUsers <= 0 || !budgetAmount}
          >
            {analyzing ? t('分析中...') : t('开始分析')}
          </Button>
          {excludedUserIds.length > 0 && (
            <Tag
              color='red'
              size='small'
              closable
              onClose={handleResetExcluded}
              style={{ cursor: 'pointer' }}
            >
              {t('已排除')} {excludedUserIds.length} {t('人')}
            </Tag>
          )}
        </Space>

        <Spin spinning={analyzing}>
          {results.length > 0 ? (
            <>
              <Table
                columns={columns}
                dataSource={results}
                rowKey='user_id'
                pagination={false}
                size='small'
                style={{ marginBottom: 12 }}
              />
              <Space
                style={{
                  width: '100%',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                }}
              >
                <Space align='center'>
                  <Text size='small' type='tertiary'>
                    {t('已选')} {selectedUserIds.length} / {results.length}
                    {' | '}
                    {t('周消费')}: {renderQuota(totalSelectedQuota, 2)}
                  </Text>
                </Space>
                <Space align='end'>
                  <Button
                    icon={<IconRefresh />}
                    size='small'
                    theme='light'
                    onClick={handleNextBatch}
                    disabled={analyzing || results.length === 0}
                  >
                    {t('换一批')}
                  </Button>
                  <div>
                    <Text
                      size='small'
                      style={{ display: 'block', marginBottom: 4 }}
                    >
                      {t('绑定过期时间(分钟)')}
                    </Text>
                    <InputNumber
                      value={expireMinutes}
                      onChange={setExpireMinutes}
                      min={0}
                      style={{ width: 120 }}
                      size='small'
                      placeholder='0'
                    />
                  </div>
                  <Button
                    icon={<IconLink />}
                    size='small'
                    theme='solid'
                    type='primary'
                    onClick={handleBind}
                    loading={binding}
                    disabled={selectedUserIds.length === 0}
                  >
                    {binding
                      ? t('绑定中...')
                      : `${t('绑定选中用户')} (${selectedUserIds.length})`}
                  </Button>
                </Space>
              </Space>
            </>
          ) : (
            !analyzing && hasAnalyzed && results.length === 0 && (
              <Empty
                description={t('未找到符合条件的用户')}
                style={{ padding: 20 }}
              />
            )
          )}
        </Spin>
    </div>
  );
};

export default UserProfileAnalysis;
