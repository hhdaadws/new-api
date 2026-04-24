/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Empty,
  InputNumber,
  Select,
  Space,
  Spin,
  TabPane,
  Tabs,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import {
  Download,
  Edit,
  Image as ImageIcon,
  RefreshCw,
  Send,
  Trash2,
  Upload,
  X,
} from 'lucide-react';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';

const { Text } = Typography;

const pageSize = 12;

const getErrorMessage = (error) =>
  error?.response?.data?.message ||
  error?.response?.data?.error?.message ||
  error;

const getFileExtension = (filename, mimeType) => {
  const ext = filename?.split('.').pop();
  if (ext && ext !== filename) return ext;
  if (mimeType === 'image/jpeg') return 'jpg';
  if (mimeType === 'image/webp') return 'webp';
  return 'png';
};

const ImageGeneration = () => {
  const { t } = useTranslation();
  const [config, setConfig] = useState(null);
  const [history, setHistory] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [generating, setGenerating] = useState(false);
  const [activeMode, setActiveMode] = useState('generation');
  const [previewUrls, setPreviewUrls] = useState({});
  const previewUrlsRef = useRef({});
  const fileInputRef = useRef(null);
  const editPreviewUrlRef = useRef('');
  const [editImageFile, setEditImageFile] = useState(null);
  const [editImagePreviewUrl, setEditImagePreviewUrl] = useState('');
  const [editSourceName, setEditSourceName] = useState('');
  const [form, setForm] = useState({
    model: '',
    group: '',
    prompt: '',
    size: '1024x1024',
    quality: 'auto',
    output_format: 'png',
    n: 1,
  });

  const modelOptions = useMemo(
    () => (config?.models || []).map((model) => ({ label: model, value: model })),
    [config?.models],
  );
  const groupOptions = useMemo(
    () => (config?.groups || []).map((group) => ({ label: group, value: group })),
    [config?.groups],
  );
  const sizeOptions = useMemo(
    () => (config?.sizes || []).map((size) => ({ label: size, value: size })),
    [config?.sizes],
  );
  const qualityOptions = useMemo(
    () =>
      (config?.qualities || []).map((quality) => ({
        label: quality,
        value: quality,
      })),
    [config?.qualities],
  );
  const outputFormatOptions = useMemo(
    () =>
      (config?.output_formats || []).map((format) => ({
        label: format,
        value: format,
      })),
    [config?.output_formats],
  );

  const updateForm = (key, value) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const revokeEditPreview = () => {
    if (editPreviewUrlRef.current) {
      URL.revokeObjectURL(editPreviewUrlRef.current);
      editPreviewUrlRef.current = '';
    }
  };

  const setEditSource = (file, sourceName) => {
    revokeEditPreview();
    const objectUrl = URL.createObjectURL(file);
    editPreviewUrlRef.current = objectUrl;
    setEditImageFile(file);
    setEditImagePreviewUrl(objectUrl);
    setEditSourceName(sourceName || file.name);
    setActiveMode('edit');
  };

  const clearEditSource = () => {
    revokeEditPreview();
    setEditImageFile(null);
    setEditImagePreviewUrl('');
    setEditSourceName('');
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };

  const loadConfig = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/image_generation/config');
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      setConfig(data);
      setForm((prev) => ({
        ...prev,
        model: prev.model || data.models?.[0] || '',
        group: prev.group || data.groups?.[0] || '',
        size: prev.size || data.defaults?.size || '1024x1024',
        quality: prev.quality || data.defaults?.quality || 'auto',
        output_format:
          prev.output_format || data.defaults?.output_format || 'png',
        n: prev.n || data.defaults?.n || 1,
      }));
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  const revokePreview = (id) => {
    const url = previewUrlsRef.current[id];
    if (url) {
      URL.revokeObjectURL(url);
      delete previewUrlsRef.current[id];
      setPreviewUrls((prev) => {
        const next = { ...prev };
        delete next[id];
        return next;
      });
    }
  };

  const loadPreview = async (item) => {
    if (!item?.id || previewUrlsRef.current[item.id]) return;
    try {
      const res = await API.get(item.url, {
        responseType: 'blob',
        disableDuplicate: true,
      });
      const objectUrl = URL.createObjectURL(res.data);
      previewUrlsRef.current[item.id] = objectUrl;
      setPreviewUrls((prev) => ({ ...prev, [item.id]: objectUrl }));
    } catch (error) {
      showError(error);
    }
  };

  const loadHistory = async (targetPage = page) => {
    setHistoryLoading(true);
    try {
      const res = await API.get('/api/image_generation/history', {
        params: { p: targetPage, page_size: pageSize },
        disableDuplicate: true,
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      setHistory(data.items || []);
      setTotal(data.total || 0);
      setPage(data.page || targetPage);
      (data.items || []).forEach(loadPreview);
    } catch (error) {
      showError(error);
    } finally {
      setHistoryLoading(false);
    }
  };

  const buildPayload = () => ({
    model: form.model,
    group: form.group,
    prompt: form.prompt.trim(),
    size: form.size,
    quality: form.quality,
    output_format: form.output_format,
    n: form.n,
  });

  const handleSuccessItems = (items, mode) => {
    setHistory((prev) => [...items, ...prev].slice(0, pageSize));
    setTotal((prev) => prev + items.length);
    items.forEach(loadPreview);
    showSuccess(mode === 'edit' ? t('改图成功') : t('生成成功'));
  };

  const submitImageRequest = async () => {
    if (!form.prompt.trim()) {
      showError(t('请输入提示词'));
      return;
    }
    if (!form.model || !form.group) {
      showError(t('请选择模型和分组'));
      return;
    }
    if (activeMode === 'edit' && !editImageFile) {
      showError(t('请选择要修改的图片'));
      return;
    }

    setGenerating(true);
    try {
      let res;
      if (activeMode === 'edit') {
        const formData = new FormData();
        formData.append('image', editImageFile);
        Object.entries(buildPayload()).forEach(([key, value]) => {
          formData.append(key, value);
        });
        res = await API.post('/pg/images/edits', formData, {
          skipErrorHandler: true,
        });
      } else {
        res = await API.post('/pg/images/generations', buildPayload(), {
          skipErrorHandler: true,
        });
      }
      const { success, message, data } = res.data;
      if (!success) {
        showError(message || t(activeMode === 'edit' ? '改图失败' : '生成失败'));
        return;
      }
      handleSuccessItems(data?.items || [], activeMode);
    } catch (error) {
      showError(getErrorMessage(error));
    } finally {
      setGenerating(false);
    }
  };

  const selectHistoryAsEditSource = async (item) => {
    try {
      const res = await API.get(item.url, {
        responseType: 'blob',
        disableDuplicate: true,
      });
      const mimeType = res.data.type || item.mime_type || 'image/png';
      const ext = getFileExtension(item.filename, mimeType);
      const file = new File([res.data], `history-${item.id}.${ext}`, {
        type: mimeType,
      });
      setEditSource(file, item.filename || `image-${item.id}.${ext}`);
      showSuccess(t('已选择历史图片'));
    } catch (error) {
      showError(error);
    }
  };

  const handleFileChange = (event) => {
    const file = event.target.files?.[0];
    if (!file) return;
    if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) {
      showError(t('仅支持 PNG、JPEG、WebP 图片'));
      event.target.value = '';
      return;
    }
    setEditSource(file, file.name);
  };

  const downloadImage = async (item) => {
    try {
      const res = await API.get(item.url, {
        responseType: 'blob',
        disableDuplicate: true,
      });
      const objectUrl = URL.createObjectURL(res.data);
      const link = document.createElement('a');
      link.href = objectUrl;
      link.download = item.filename || `image-${item.id}.png`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(objectUrl);
    } catch (error) {
      showError(error);
    }
  };

  const deleteImage = async (item) => {
    try {
      const res = await API.delete(`/api/image_generation/${item.id}`);
      const { success, message } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      revokePreview(item.id);
      setHistory((prev) =>
        prev.filter((historyItem) => historyItem.id !== item.id),
      );
      setTotal((prev) => Math.max(0, prev - 1));
      showSuccess(t('删除成功'));
    } catch (error) {
      showError(error);
    }
  };

  useEffect(() => {
    loadConfig();
    loadHistory(1);
    return () => {
      Object.values(previewUrlsRef.current).forEach((url) =>
        URL.revokeObjectURL(url),
      );
      previewUrlsRef.current = {};
      revokeEditPreview();
    };
  }, []);

  const disabled =
    !config?.enabled || !modelOptions.length || !groupOptions.length;
  const maxPage = Math.max(1, Math.ceil(total / pageSize));

  return (
    <div className='mt-[60px] px-2 pb-6'>
      <Spin spinning={loading}>
        <div className='mx-auto flex w-full max-w-[1280px] flex-col gap-4'>
          <div className='flex flex-col gap-2 md:flex-row md:items-end md:justify-between'>
            <div>
              <Typography.Title heading={4} style={{ margin: 0 }}>
                {t('图像生成')}
              </Typography.Title>
            </div>
            <Button
              icon={<RefreshCw size={16} />}
              onClick={() => loadHistory(page)}
              loading={historyLoading}
            >
              {t('刷新')}
            </Button>
          </div>

          {!config?.enabled && (
            <Card>
              <Empty title={t('图像生成页面未启用')} />
            </Card>
          )}

          {config?.enabled && (
            <div className='grid grid-cols-1 gap-4 lg:grid-cols-[360px_minmax(0,1fr)]'>
              <Card bodyStyle={{ padding: 16 }}>
                <Tabs
                  type='button'
                  activeKey={activeMode}
                  onChange={setActiveMode}
                  style={{ marginBottom: 16 }}
                >
                  <TabPane tab={t('生成图片')} itemKey='generation' />
                  <TabPane tab={t('上传改图')} itemKey='edit' />
                </Tabs>
                <div className='flex flex-col gap-4'>
                  {activeMode === 'edit' && (
                    <div>
                      <Text strong>{t('图像来源')}</Text>
                      <div
                        className='mt-2 flex aspect-square w-full items-center justify-center overflow-hidden rounded-md border border-dashed'
                        style={{
                          borderColor: 'var(--semi-color-border)',
                          backgroundColor: 'var(--semi-color-fill-0)',
                        }}
                      >
                        {editImagePreviewUrl ? (
                          <img
                            src={editImagePreviewUrl}
                            alt={editSourceName}
                            className='h-full w-full object-contain'
                          />
                        ) : (
                          <ImageIcon size={40} color='var(--semi-color-text-2)' />
                        )}
                      </div>
                      {editSourceName && (
                        <Text
                          type='secondary'
                          size='small'
                          ellipsis={{ showTooltip: true }}
                          className='mt-2 block'
                        >
                          {editSourceName}
                        </Text>
                      )}
                      <div className='mt-3 flex gap-2'>
                        <input
                          ref={fileInputRef}
                          type='file'
                          accept='image/png,image/jpeg,image/webp'
                          className='hidden'
                          onChange={handleFileChange}
                        />
                        <Button
                          icon={<Upload size={16} />}
                          onClick={() => fileInputRef.current?.click()}
                          disabled={disabled || generating}
                        >
                          {t('上传图片')}
                        </Button>
                        {editImageFile && (
                          <Button
                            icon={<X size={16} />}
                            onClick={clearEditSource}
                            disabled={generating}
                          >
                            {t('清除')}
                          </Button>
                        )}
                      </div>
                    </div>
                  )}

                  <div>
                    <Text strong>{t('提示词')}</Text>
                    <TextArea
                      autosize={{ minRows: 6, maxRows: 12 }}
                      value={form.prompt}
                      onChange={(value) => updateForm('prompt', value)}
                      placeholder={
                        activeMode === 'edit'
                          ? t('描述你想如何修改这张图片')
                          : t('描述你想生成的图片')
                      }
                      disabled={disabled || generating}
                      style={{ marginTop: 8 }}
                    />
                  </div>

                  <div className='grid grid-cols-1 gap-3'>
                    <div>
                      <Text strong>{t('模型')}</Text>
                      <Select
                        value={form.model}
                        optionList={modelOptions}
                        onChange={(value) => updateForm('model', value)}
                        disabled={disabled || generating}
                        style={{ width: '100%', marginTop: 8 }}
                      />
                    </div>
                    <div>
                      <Text strong>{t('分组')}</Text>
                      <Select
                        value={form.group}
                        optionList={groupOptions}
                        onChange={(value) => updateForm('group', value)}
                        disabled={disabled || generating}
                        style={{ width: '100%', marginTop: 8 }}
                      />
                    </div>
                    <div className='grid grid-cols-2 gap-3'>
                      <div>
                        <Text strong>{t('尺寸')}</Text>
                        <Select
                          value={form.size}
                          optionList={sizeOptions}
                          onChange={(value) => updateForm('size', value)}
                          disabled={disabled || generating}
                          style={{ width: '100%', marginTop: 8 }}
                        />
                      </div>
                      <div>
                        <Text strong>{t('质量')}</Text>
                        <Select
                          value={form.quality}
                          optionList={qualityOptions}
                          onChange={(value) => updateForm('quality', value)}
                          disabled={disabled || generating}
                          style={{ width: '100%', marginTop: 8 }}
                        />
                      </div>
                    </div>
                    <div className='grid grid-cols-2 gap-3'>
                      <div>
                        <Text strong>{t('格式')}</Text>
                        <Select
                          value={form.output_format}
                          optionList={outputFormatOptions}
                          onChange={(value) => updateForm('output_format', value)}
                          disabled={disabled || generating}
                          style={{ width: '100%', marginTop: 8 }}
                        />
                      </div>
                      <div>
                        <Text strong>{t('数量')}</Text>
                        <InputNumber
                          min={1}
                          max={config?.defaults?.max_n || 4}
                          value={form.n}
                          onChange={(value) => updateForm('n', value || 1)}
                          disabled={disabled || generating}
                          style={{ width: '100%', marginTop: 8 }}
                        />
                      </div>
                    </div>
                  </div>

                  <Button
                    type='primary'
                    icon={<Send size={16} />}
                    loading={generating}
                    disabled={disabled}
                    onClick={submitImageRequest}
                    block
                  >
                    {activeMode === 'edit' ? t('提交改图') : t('生成图片')}
                  </Button>
                </div>
              </Card>

              <div className='flex min-w-0 flex-col gap-3'>
                <Spin spinning={historyLoading || generating}>
                  {history.length === 0 ? (
                    <Card>
                      <Empty
                        image={<ImageIcon size={42} />}
                        title={t('暂无图片')}
                      />
                    </Card>
                  ) : (
                    <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3'>
                      {history.map((item) => (
                        <Card
                          key={item.id}
                          bodyStyle={{ padding: 0 }}
                          className='overflow-hidden'
                        >
                          <div className='aspect-square w-full bg-[var(--semi-color-fill-0)]'>
                            {previewUrls[item.id] ? (
                              <img
                                src={previewUrls[item.id]}
                                alt={item.prompt}
                                className='h-full w-full object-contain'
                              />
                            ) : (
                              <div className='flex h-full w-full items-center justify-center text-[var(--semi-color-text-2)]'>
                                <ImageIcon size={32} />
                              </div>
                            )}
                          </div>
                          <div className='flex flex-col gap-3 p-3'>
                            <div className='line-clamp-2 min-h-[40px] text-sm'>
                              {item.prompt}
                            </div>
                            <div className='flex flex-wrap gap-2'>
                              <Tag color={item.kind === 'edit' ? 'green' : 'blue'}>
                                {item.kind === 'edit' ? t('改图') : t('生成')}
                              </Tag>
                              <Tag color='blue'>{item.model}</Tag>
                              <Tag>{item.group}</Tag>
                              <Tag>{item.size}</Tag>
                            </div>
                            {item.revised_prompt && (
                              <Text
                                type='secondary'
                                size='small'
                                ellipsis={{ showTooltip: true }}
                              >
                                {item.revised_prompt}
                              </Text>
                            )}
                            <div className='flex items-center justify-between gap-2'>
                              <Text type='secondary' size='small'>
                                {timestamp2string(item.created_at)}
                              </Text>
                              <Space>
                                <Button
                                  size='small'
                                  icon={<Edit size={14} />}
                                  onClick={() => selectHistoryAsEditSource(item)}
                                >
                                  {t('改图')}
                                </Button>
                                <Button
                                  size='small'
                                  icon={<Download size={14} />}
                                  onClick={() => downloadImage(item)}
                                />
                                <Button
                                  size='small'
                                  type='danger'
                                  theme='borderless'
                                  icon={<Trash2 size={14} />}
                                  onClick={() => deleteImage(item)}
                                />
                              </Space>
                            </div>
                          </div>
                        </Card>
                      ))}
                    </div>
                  )}
                </Spin>

                {total > pageSize && (
                  <div className='flex items-center justify-end gap-2'>
                    <Button
                      disabled={page <= 1}
                      onClick={() => loadHistory(page - 1)}
                    >
                      {t('上一页')}
                    </Button>
                    <Text type='secondary'>
                      {page} / {maxPage}
                    </Text>
                    <Button
                      disabled={page >= maxPage}
                      onClick={() => loadHistory(page + 1)}
                    >
                      {t('下一页')}
                    </Button>
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      </Spin>
    </div>
  );
};

export default ImageGeneration;
