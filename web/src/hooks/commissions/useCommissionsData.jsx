import { useState, useEffect } from 'react';
import { API, showError, showSuccess } from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';
import { COMMISSION_STATUS } from '../../constants/commission.constants';
import { Modal } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { useTableCompactMode } from '../common/useTableCompactMode';

export const useCommissionsData = () => {
  const { t } = useTranslation();

  const [commissions, setCommissions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [searching, setSearching] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [totalCount, setTotalCount] = useState(0);
  const [selectedKeys, setSelectedKeys] = useState([]);
  const [statusFilter, setStatusFilter] = useState(0);

  const [formApi, setFormApi] = useState(null);
  const [compactMode, setCompactMode] = useTableCompactMode('commissions');

  const formInitValues = {
    searchKeyword: '',
  };

  const getFormValues = () => {
    const formValues = formApi ? formApi.getValues() : {};
    return {
      searchKeyword: formValues.searchKeyword || '',
    };
  };

  const loadCommissions = async (page = 1, size = pageSize, status = statusFilter) => {
    setLoading(true);
    try {
      let url = `/api/commission/?p=${page}&page_size=${size}`;
      if (status > 0) {
        url += `&status=${status}`;
      }
      const { searchKeyword } = getFormValues();
      if (searchKeyword) {
        url += `&keyword=${searchKeyword}`;
      }
      const res = await API.get(url);
      const { success, message, data } = res.data;
      if (success) {
        setActivePage(data.page <= 0 ? 1 : data.page);
        setTotalCount(data.total);
        setCommissions(data.items || []);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
    setLoading(false);
  };

  const searchCommissions = async () => {
    setSearching(true);
    await loadCommissions(1, pageSize, statusFilter);
    setSearching(false);
  };

  const approveCommission = async (id) => {
    setLoading(true);
    try {
      const res = await API.post('/api/commission/approve', { id });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('操作成功完成！'));
        await refresh();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
    setLoading(false);
  };

  const rejectCommission = async (id, remark) => {
    setLoading(true);
    try {
      const res = await API.post('/api/commission/reject', { id, remark });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('操作成功完成！'));
        await refresh();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
    setLoading(false);
  };

  const batchApproveCommissions = async () => {
    if (selectedKeys.length === 0) {
      showError(t('请至少选择一条记录'));
      return;
    }
    const pendingKeys = selectedKeys.filter(
      (r) => r.status === COMMISSION_STATUS.PENDING,
    );
    if (pendingKeys.length === 0) {
      showError(t('所选记录中没有待审核的记录'));
      return;
    }
    Modal.confirm({
      title: t('确定批量通过所选返佣记录？'),
      content: t('将通过 {{count}} 条待审核记录', { count: pendingKeys.length }),
      onOk: async () => {
        setLoading(true);
        try {
          const ids = pendingKeys.map((r) => r.id);
          const res = await API.post('/api/commission/batch/approve', { ids });
          const { success, message, data } = res.data;
          if (success) {
            showSuccess(t('成功通过 {{count}} 条记录', { count: data }));
            setSelectedKeys([]);
            await refresh();
          } else {
            showError(message);
          }
        } catch (error) {
          showError(error.message);
        }
        setLoading(false);
      },
    });
  };

  const batchRejectCommissions = async (remark) => {
    if (selectedKeys.length === 0) {
      showError(t('请至少选择一条记录'));
      return;
    }
    const pendingKeys = selectedKeys.filter(
      (r) => r.status === COMMISSION_STATUS.PENDING,
    );
    if (pendingKeys.length === 0) {
      showError(t('所选记录中没有待审核的记录'));
      return;
    }
    setLoading(true);
    try {
      const ids = pendingKeys.map((r) => r.id);
      const res = await API.post('/api/commission/batch/reject', {
        ids,
        remark: remark || '',
      });
      const { success, message, data } = res.data;
      if (success) {
        showSuccess(t('成功拒绝 {{count}} 条记录', { count: data }));
        setSelectedKeys([]);
        await refresh();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
    setLoading(false);
  };

  const refresh = async (page = activePage) => {
    await loadCommissions(page, pageSize, statusFilter);
  };

  const handlePageChange = (page) => {
    setActivePage(page);
    loadCommissions(page, pageSize, statusFilter);
  };

  const handlePageSizeChange = (size) => {
    setPageSize(size);
    setActivePage(1);
    loadCommissions(1, size, statusFilter);
  };

  const handleStatusFilterChange = (status) => {
    setStatusFilter(status);
    setActivePage(1);
    loadCommissions(1, pageSize, status);
  };

  const rowSelection = {
    onSelect: () => {},
    onSelectAll: () => {},
    onChange: (selectedRowKeys, selectedRows) => {
      setSelectedKeys(selectedRows);
    },
  };

  const handleRow = (record) => {
    if (record.status === COMMISSION_STATUS.REJECTED) {
      return {
        style: {
          background: 'var(--semi-color-disabled-border)',
        },
      };
    }
    return {};
  };

  useEffect(() => {
    loadCommissions(1, pageSize, statusFilter);
  }, []);

  return {
    commissions,
    loading,
    searching,
    activePage,
    pageSize,
    totalCount,
    selectedKeys,
    statusFilter,
    formApi,
    formInitValues,
    compactMode,
    setCompactMode,
    loadCommissions,
    searchCommissions,
    approveCommission,
    rejectCommission,
    batchApproveCommissions,
    batchRejectCommissions,
    refresh,
    setActivePage,
    setPageSize,
    setSelectedKeys,
    setFormApi,
    setLoading,
    handlePageChange,
    handlePageSizeChange,
    handleStatusFilterChange,
    rowSelection,
    handleRow,
    t,
  };
};
