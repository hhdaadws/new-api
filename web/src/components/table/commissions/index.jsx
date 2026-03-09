import React from 'react';
import CardPro from '../../common/ui/CardPro';
import CommissionsTable from './CommissionsTable';
import CommissionsActions from './CommissionsActions';
import CommissionsFilters from './CommissionsFilters';
import CommissionsDescription from './CommissionsDescription';
import { useCommissionsData } from '../../../hooks/commissions/useCommissionsData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

const CommissionsPage = () => {
  const commissionsData = useCommissionsData();
  const isMobile = useIsMobile();

  const {
    selectedKeys,
    batchApproveCommissions,
    formInitValues,
    setFormApi,
    searchCommissions,
    loading,
    searching,
    statusFilter,
    handleStatusFilterChange,
    compactMode,
    setCompactMode,
    t,
  } = commissionsData;

  return (
    <CardPro
      type='type1'
      descriptionArea={
        <CommissionsDescription
          compactMode={compactMode}
          setCompactMode={setCompactMode}
          t={t}
        />
      }
      actionsArea={
        <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
          <CommissionsActions
            selectedKeys={selectedKeys}
            batchApproveCommissions={batchApproveCommissions}
            commissionsData={commissionsData}
            t={t}
          />

          <div className='w-full md:w-full lg:w-auto order-1 md:order-2'>
            <CommissionsFilters
              formInitValues={formInitValues}
              setFormApi={setFormApi}
              searchCommissions={searchCommissions}
              loading={loading}
              searching={searching}
              statusFilter={statusFilter}
              handleStatusFilterChange={handleStatusFilterChange}
              t={t}
            />
          </div>
        </div>
      }
      paginationArea={createCardProPagination({
        currentPage: commissionsData.activePage,
        pageSize: commissionsData.pageSize,
        total: commissionsData.totalCount,
        onPageChange: commissionsData.handlePageChange,
        onPageSizeChange: commissionsData.handlePageSizeChange,
        isMobile: isMobile,
        t: commissionsData.t,
      })}
      t={commissionsData.t}
    >
      <CommissionsTable {...commissionsData} />
    </CardPro>
  );
};

export default CommissionsPage;
