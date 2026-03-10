import React from 'react';
import { Typography } from '@douyinfe/semi-ui';
import { TicketCheck } from 'lucide-react';
import CompactModeToggle from '../../common/ui/CompactModeToggle';

const { Text } = Typography;

const TicketsDescription = ({ compactMode, setCompactMode, t }) => {
  return (
    <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
      <div className='flex items-center text-blue-500'>
        <TicketCheck size={16} className='mr-2' />
        <Text>{t('工单系统')}</Text>
      </div>

      <CompactModeToggle
        compactMode={compactMode}
        setCompactMode={setCompactMode}
        t={t}
      />
    </div>
  );
};

export default TicketsDescription;
