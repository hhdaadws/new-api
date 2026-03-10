import { useState, useEffect } from 'react';
import { API, showError, showSuccess, isAdmin } from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';
import { useTranslation } from 'react-i18next';
import { useTableCompactMode } from '../common/useTableCompactMode';

export const useTicketsData = () => {
  const { t } = useTranslation();
  const admin = isAdmin();

  const [tickets, setTickets] = useState([]);
  const [loading, setLoading] = useState(true);
  const [searching, setSearching] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [totalCount, setTotalCount] = useState(0);
  const [statusFilter, setStatusFilter] = useState(0);
  const [priorityFilter, setPriorityFilter] = useState(0);
  const [categoryFilter, setCategoryFilter] = useState(0);

  const [formApi, setFormApi] = useState(null);
  const [compactMode, setCompactMode] = useTableCompactMode('tickets');

  // 创建工单弹窗
  const [showCreateModal, setShowCreateModal] = useState(false);
  // 工单详情弹窗
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [currentTicket, setCurrentTicket] = useState(null);
  // 分配工单弹窗
  const [showAssignModal, setShowAssignModal] = useState(false);
  const [assigningTicket, setAssigningTicket] = useState(null);

  const formInitValues = {
    searchKeyword: '',
  };

  const getFormValues = () => {
    const formValues = formApi ? formApi.getValues() : {};
    return {
      searchKeyword: formValues.searchKeyword || '',
    };
  };

  const loadTickets = async (
    page = 1,
    size = pageSize,
    status = statusFilter,
    priority = priorityFilter,
    category = categoryFilter,
  ) => {
    setLoading(true);
    try {
      const basePath = admin ? '/api/ticket/' : '/api/ticket/self';
      let url = `${basePath}?p=${page}&page_size=${size}`;
      if (status > 0) {
        url += `&status=${status}`;
      }
      if (admin && priority > 0) {
        url += `&priority=${priority}`;
      }
      if (admin && category > 0) {
        url += `&category=${category}`;
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
        setTickets(data.items || []);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
    setLoading(false);
  };

  const searchTickets = async () => {
    setSearching(true);
    await loadTickets(1, pageSize, statusFilter, priorityFilter, categoryFilter);
    setSearching(false);
  };

  const createTicket = async (data) => {
    try {
      const res = await API.post('/api/ticket/', data);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('工单创建成功'));
        setShowCreateModal(false);
        await refresh();
        return true;
      } else {
        showError(message);
        return false;
      }
    } catch (error) {
      showError(error.message);
      return false;
    }
  };

  const closeTicket = async (id) => {
    try {
      const res = await API.post(`/api/ticket/self/${id}/close`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('工单已关闭'));
        await refresh();
        return true;
      } else {
        showError(message);
        return false;
      }
    } catch (error) {
      showError(error.message);
      return false;
    }
  };

  const rateTicket = async (id, rating) => {
    try {
      const res = await API.post(`/api/ticket/self/${id}/rate`, { rating });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('评价成功'));
        await refresh();
        return true;
      } else {
        showError(message);
        return false;
      }
    } catch (error) {
      showError(error.message);
      return false;
    }
  };

  const addMessage = async (ticketId, content) => {
    try {
      const basePath = admin
        ? `/api/ticket/${ticketId}/message`
        : `/api/ticket/self/${ticketId}/message`;
      const res = await API.post(basePath, { content });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('回复发送成功'));
        return true;
      } else {
        showError(message);
        return false;
      }
    } catch (error) {
      showError(error.message);
      return false;
    }
  };

  const updateTicketStatus = async (id, status) => {
    try {
      const res = await API.put(`/api/ticket/${id}/status`, { status });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('状态更新成功'));
        await refresh();
        return true;
      } else {
        showError(message);
        return false;
      }
    } catch (error) {
      showError(error.message);
      return false;
    }
  };

  const assignTicket = async (ticketId, adminId, adminName) => {
    try {
      const res = await API.put(`/api/ticket/${ticketId}/assign`, {
        admin_id: adminId,
        admin_name: adminName,
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('分配成功'));
        setShowAssignModal(false);
        await refresh();
        return true;
      } else {
        showError(message);
        return false;
      }
    } catch (error) {
      showError(error.message);
      return false;
    }
  };

  const loadTicketDetail = async (ticketId) => {
    try {
      const basePath = admin
        ? `/api/ticket/${ticketId}`
        : `/api/ticket/self/${ticketId}`;
      const res = await API.get(basePath);
      const { success, message, data } = res.data;
      if (success) {
        return data;
      } else {
        showError(message);
        return null;
      }
    } catch (error) {
      showError(error.message);
      return null;
    }
  };

  const openTicketDetail = async (ticket) => {
    setCurrentTicket(ticket);
    setShowDetailModal(true);
  };

  const openAssignModal = (ticket) => {
    setAssigningTicket(ticket);
    setShowAssignModal(true);
  };

  const refresh = async (page = activePage) => {
    await loadTickets(page, pageSize, statusFilter, priorityFilter, categoryFilter);
  };

  const handlePageChange = (page) => {
    setActivePage(page);
    loadTickets(page, pageSize, statusFilter, priorityFilter, categoryFilter);
  };

  const handlePageSizeChange = (size) => {
    setPageSize(size);
    setActivePage(1);
    loadTickets(1, size, statusFilter, priorityFilter, categoryFilter);
  };

  const handleStatusFilterChange = (status) => {
    setStatusFilter(status);
    setActivePage(1);
    loadTickets(1, pageSize, status, priorityFilter, categoryFilter);
  };

  const handlePriorityFilterChange = (priority) => {
    setPriorityFilter(priority);
    setActivePage(1);
    loadTickets(1, pageSize, statusFilter, priority, categoryFilter);
  };

  const handleCategoryFilterChange = (category) => {
    setCategoryFilter(category);
    setActivePage(1);
    loadTickets(1, pageSize, statusFilter, priorityFilter, category);
  };

  useEffect(() => {
    loadTickets(1, pageSize, statusFilter, priorityFilter, categoryFilter);
  }, []);

  return {
    tickets,
    loading,
    searching,
    activePage,
    pageSize,
    totalCount,
    statusFilter,
    priorityFilter,
    categoryFilter,
    formApi,
    formInitValues,
    compactMode,
    setCompactMode,
    admin,
    // 弹窗状态
    showCreateModal,
    setShowCreateModal,
    showDetailModal,
    setShowDetailModal,
    currentTicket,
    setCurrentTicket,
    showAssignModal,
    setShowAssignModal,
    assigningTicket,
    setAssigningTicket,
    // 方法
    loadTickets,
    searchTickets,
    createTicket,
    closeTicket,
    rateTicket,
    addMessage,
    updateTicketStatus,
    assignTicket,
    loadTicketDetail,
    openTicketDetail,
    openAssignModal,
    refresh,
    setActivePage,
    setPageSize,
    setFormApi,
    setLoading,
    handlePageChange,
    handlePageSizeChange,
    handleStatusFilterChange,
    handlePriorityFilterChange,
    handleCategoryFilterChange,
    t,
  };
};
