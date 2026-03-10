import React from 'react';
import { Button } from '@douyinfe/semi-ui';
import { Plus } from 'lucide-react';

const TicketsActions = ({ ticketsData, t }) => {
  const { setShowCreateModal } = ticketsData;

  return (
    <div className='flex flex-wrap gap-2 w-full md:w-auto order-2 md:order-1'>
      <Button
        type='primary'
        className='flex-1 md:flex-initial'
        onClick={() => setShowCreateModal(true)}
        size='small'
        icon={<Plus size={14} />}
      >
        {t('创建工单')}
      </Button>
    </div>
  );
};

export default TicketsActions;
