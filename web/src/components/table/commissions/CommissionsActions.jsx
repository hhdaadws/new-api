import React, { useState } from 'react';
import { Button } from '@douyinfe/semi-ui';
import RejectCommissionModal from './modals/RejectCommissionModal';

const CommissionsActions = ({
  selectedKeys,
  batchApproveCommissions,
  commissionsData,
  t,
}) => {
  const [showBatchRejectModal, setShowBatchRejectModal] = useState(false);

  const handleBatchReject = async (remark) => {
    await commissionsData.batchRejectCommissions(remark);
    setShowBatchRejectModal(false);
  };

  return (
    <>
      <div className='flex flex-wrap gap-2 w-full md:w-auto order-2 md:order-1'>
        <Button
          type='primary'
          className='flex-1 md:flex-initial'
          onClick={batchApproveCommissions}
          disabled={selectedKeys.length === 0}
          size='small'
        >
          {t('批量通过')}
        </Button>

        <Button
          type='danger'
          className='flex-1 md:flex-initial'
          onClick={() => setShowBatchRejectModal(true)}
          disabled={selectedKeys.length === 0}
          size='small'
        >
          {t('批量拒绝')}
        </Button>
      </div>

      <RejectCommissionModal
        visible={showBatchRejectModal}
        onCancel={() => setShowBatchRejectModal(false)}
        onConfirm={handleBatchReject}
        isBatch={true}
        count={selectedKeys.length}
        t={t}
      />
    </>
  );
};

export default CommissionsActions;
