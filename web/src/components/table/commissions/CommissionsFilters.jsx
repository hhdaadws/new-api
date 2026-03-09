import React, { useRef } from 'react';
import { Form, Button, Select } from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';
import {
  COMMISSION_STATUS,
  COMMISSION_STATUS_MAP,
} from '../../../constants/commission.constants';

const CommissionsFilters = ({
  formInitValues,
  setFormApi,
  searchCommissions,
  loading,
  searching,
  statusFilter,
  handleStatusFilterChange,
  t,
}) => {
  const formApiRef = useRef(null);

  const handleReset = () => {
    if (formApiRef.current) {
      formApiRef.current.reset();
    }
    handleStatusFilterChange(0);
    setTimeout(() => {
      searchCommissions();
    }, 100);
  };

  const statusOptions = [
    { value: 0, label: t('全部状态') },
    ...Object.entries(COMMISSION_STATUS_MAP).map(([key, config]) => ({
      value: parseInt(key),
      label: t(config.text),
    })),
  ];

  return (
    <Form
      initValues={formInitValues}
      getFormApi={(api) => {
        setFormApi(api);
        formApiRef.current = api;
      }}
      onSubmit={searchCommissions}
      allowEmpty={true}
      autoComplete='off'
      layout='horizontal'
      trigger='change'
      stopValidateWithError={false}
      className='w-full md:w-auto order-1 md:order-2'
    >
      <div className='flex flex-col md:flex-row items-center gap-2 w-full md:w-auto'>
        <Select
          value={statusFilter}
          onChange={handleStatusFilterChange}
          optionList={statusOptions}
          size='small'
          style={{ width: 120 }}
        />
        <div className='relative w-full md:w-64'>
          <Form.Input
            field='searchKeyword'
            prefix={<IconSearch />}
            placeholder={t('搜索用户名或订单号')}
            showClear
            pure
            size='small'
          />
        </div>
        <div className='flex gap-2 w-full md:w-auto'>
          <Button
            type='tertiary'
            htmlType='submit'
            loading={loading || searching}
            className='flex-1 md:flex-initial md:w-auto'
            size='small'
          >
            {t('查询')}
          </Button>
          <Button
            type='tertiary'
            onClick={handleReset}
            className='flex-1 md:flex-initial md:w-auto'
            size='small'
          >
            {t('重置')}
          </Button>
        </div>
      </div>
    </Form>
  );
};

export default CommissionsFilters;
