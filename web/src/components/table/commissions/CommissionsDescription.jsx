import React from 'react';
import { Typography } from '@douyinfe/semi-ui';
import { HandCoins } from 'lucide-react';
import CompactModeToggle from '../../common/ui/CompactModeToggle';

const { Text } = Typography;

const CommissionsDescription = ({ compactMode, setCompactMode, t }) => {
  return (
    <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
      <div className='flex items-center text-orange-500'>
        <HandCoins size={16} className='mr-2' />
        <Text>{t('返佣审核')}</Text>
      </div>

      <CompactModeToggle
        compactMode={compactMode}
        setCompactMode={setCompactMode}
        t={t}
      />
    </div>
  );
};

export default CommissionsDescription;
