import React, { useMemo, useState } from 'react';
import { Empty } from '@douyinfe/semi-ui';
import CardTable from '../../common/ui/CardTable';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { getCommissionsColumns } from './CommissionsColumnDefs';
import RejectCommissionModal from './modals/RejectCommissionModal';

const CommissionsTable = (commissionsData) => {
  const {
    commissions,
    loading,
    activePage,
    pageSize,
    totalCount,
    compactMode,
    handlePageChange,
    handlePageSizeChange,
    rowSelection,
    handleRow,
    approveCommission,
    rejectCommission,
    t,
  } = commissionsData;

  const [showRejectModal, setShowRejectModal] = useState(false);
  const [rejectingRecord, setRejectingRecord] = useState(null);

  const showRejectCommissionModal = (record) => {
    setRejectingRecord(record);
    setShowRejectModal(true);
  };

  const handleRejectConfirm = async (remark) => {
    if (rejectingRecord) {
      await rejectCommission(rejectingRecord.id, remark);
      setShowRejectModal(false);
      setRejectingRecord(null);
    }
  };

  const columns = useMemo(() => {
    return getCommissionsColumns({
      t,
      approveCommission,
      showRejectCommissionModal,
    });
  }, [t, approveCommission]);

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
        dataSource={commissions}
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
        rowSelection={rowSelection}
        onRow={handleRow}
        empty={
          <Empty
            image={
              <IllustrationNoResult style={{ width: 150, height: 150 }} />
            }
            darkModeImage={
              <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
            }
            description={t('搜索无结果')}
            style={{ padding: 30 }}
          />
        }
        className='rounded-xl overflow-hidden'
        size='middle'
      />

      <RejectCommissionModal
        visible={showRejectModal}
        onCancel={() => {
          setShowRejectModal(false);
          setRejectingRecord(null);
        }}
        onConfirm={handleRejectConfirm}
        record={rejectingRecord}
        t={t}
      />
    </>
  );
};

export default CommissionsTable;
