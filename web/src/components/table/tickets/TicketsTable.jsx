import React, { useMemo } from 'react';
import { Empty } from '@douyinfe/semi-ui';
import CardTable from '../../common/ui/CardTable';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { getTicketsColumns } from './TicketsColumnDefs';
import CreateTicketModal from './modals/CreateTicketModal';
import TicketDetailModal from './modals/TicketDetailModal';
import AssignTicketModal from './modals/AssignTicketModal';

const TicketsTable = (ticketsData) => {
  const {
    tickets,
    loading,
    activePage,
    pageSize,
    totalCount,
    compactMode,
    admin,
    handlePageChange,
    handlePageSizeChange,
    openTicketDetail,
    openAssignModal,
    updateTicketStatus,
    // 弹窗相关
    showCreateModal,
    setShowCreateModal,
    createTicket,
    showDetailModal,
    setShowDetailModal,
    currentTicket,
    setCurrentTicket,
    showAssignModal,
    setShowAssignModal,
    assigningTicket,
    setAssigningTicket,
    assignTicket,
    loadTicketDetail,
    addMessage,
    closeTicket,
    rateTicket,
    refresh,
    t,
  } = ticketsData;

  const columns = useMemo(() => {
    return getTicketsColumns({
      t,
      admin,
      openTicketDetail,
      openAssignModal,
      updateTicketStatus,
    });
  }, [t, admin, openTicketDetail, openAssignModal, updateTicketStatus]);

  const tableColumns = useMemo(() => {
    return compactMode
      ? columns.map((col) => {
          if (col.dataIndex === 'operate') {
            const { fixed, ...rest } = col;
            return rest;
          }
          return col;
        })
      : columns;
  }, [compactMode, columns]);

  return (
    <>
      <CardTable
        columns={tableColumns}
        dataSource={tickets}
        scroll={compactMode ? undefined : { x: 'max-content' }}
        pagination={{
          currentPage: activePage,
          pageSize: pageSize,
          total: totalCount,
          showSizeChanger: true,
          pageSizeOptions: [10, 20, 50, 100],
          onPageSizeChange: handlePageSizeChange,
          onPageChange: handlePageChange,
        }}
        hidePagination={true}
        loading={loading}
        empty={
          <Empty
            image={
              <IllustrationNoResult style={{ width: 150, height: 150 }} />
            }
            darkModeImage={
              <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
            }
            description={t('暂无工单')}
            style={{ padding: 30 }}
          />
        }
        className='rounded-xl overflow-hidden'
        size='middle'
      />

      <CreateTicketModal
        visible={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onSubmit={createTicket}
        t={t}
      />

      <TicketDetailModal
        visible={showDetailModal}
        onClose={() => {
          setShowDetailModal(false);
          setCurrentTicket(null);
        }}
        ticket={currentTicket}
        admin={admin}
        loadTicketDetail={loadTicketDetail}
        addMessage={addMessage}
        closeTicket={closeTicket}
        rateTicket={rateTicket}
        updateTicketStatus={updateTicketStatus}
        openAssignModal={openAssignModal}
        refresh={refresh}
        t={t}
      />

      <AssignTicketModal
        visible={showAssignModal}
        onClose={() => {
          setShowAssignModal(false);
          setAssigningTicket(null);
        }}
        ticket={assigningTicket}
        onAssign={assignTicket}
        t={t}
      />
    </>
  );
};

export default TicketsTable;
