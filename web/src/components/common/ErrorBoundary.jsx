import React from 'react';
import { Button, Empty } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';

class ErrorBoundaryInner extends React.Component {
  constructor(props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error) {
    return { hasError: true, error };
  }

  componentDidCatch(error, errorInfo) {
    console.error('ErrorBoundary caught:', error, errorInfo);
  }

  handleReset = () => {
    this.setState({ hasError: false, error: null });
  };

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback({
          error: this.state.error,
          reset: this.handleReset,
        });
      }
      const { t } = this.props;
      return (
        <div className='flex flex-col items-center justify-center p-8 gap-4'>
          <Empty description={t('页面出现错误')} />
          <Button type='primary' onClick={this.handleReset}>
            {t('重试')}
          </Button>
        </div>
      );
    }
    return this.props.children;
  }
}

const ErrorBoundary = (props) => {
  const { t } = useTranslation();
  return <ErrorBoundaryInner t={t} {...props} />;
};

export default ErrorBoundary;
