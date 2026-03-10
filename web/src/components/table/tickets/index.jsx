import React from 'react';
import CardPro from '../../common/ui/CardPro';
import TicketsTable from './TicketsTable';
import TicketsActions from './TicketsActions';
import TicketsFilters from './TicketsFilters';
import TicketsDescription from './TicketsDescription';
import { useTicketsData } from '../../../hooks/tickets/useTicketsData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

const TicketsPage = () => {
  const ticketsData = useTicketsData();
  const isMobile = useIsMobile();

  const {
    formInitValues,
    setFormApi,
    searchTickets,
    loading,
    searching,
    statusFilter,
    priorityFilter,
    categoryFilter,
    handleStatusFilterChange,
    handlePriorityFilterChange,
    handleCategoryFilterChange,
    compactMode,
    setCompactMode,
    admin,
    t,
  } = ticketsData;

  return (
    <CardPro
      type='type1'
      descriptionArea={
        <TicketsDescription
          compactMode={compactMode}
          setCompactMode={setCompactMode}
          t={t}
        />
      }
      actionsArea={
        <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
          <TicketsActions ticketsData={ticketsData} t={t} />

          <div className='w-full md:w-full lg:w-auto order-1 md:order-2'>
            <TicketsFilters
              formInitValues={formInitValues}
              setFormApi={setFormApi}
              searchTickets={searchTickets}
              loading={loading}
              searching={searching}
              statusFilter={statusFilter}
              priorityFilter={priorityFilter}
              categoryFilter={categoryFilter}
              handleStatusFilterChange={handleStatusFilterChange}
              handlePriorityFilterChange={handlePriorityFilterChange}
              handleCategoryFilterChange={handleCategoryFilterChange}
              admin={admin}
              t={t}
            />
          </div>
        </div>
      }
      paginationArea={createCardProPagination({
        currentPage: ticketsData.activePage,
        pageSize: ticketsData.pageSize,
        total: ticketsData.totalCount,
        onPageChange: ticketsData.handlePageChange,
        onPageSizeChange: ticketsData.handlePageSizeChange,
        isMobile: isMobile,
        t: ticketsData.t,
      })}
      t={ticketsData.t}
    >
      <TicketsTable {...ticketsData} />
    </CardPro>
  );
};

export default TicketsPage;
