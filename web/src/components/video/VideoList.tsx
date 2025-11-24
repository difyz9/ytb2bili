'use client';

import { useState, useEffect } from 'react';
import { Play, RefreshCw, Trash2 } from 'lucide-react';
import StatusBadge from '@/components/ui/StatusBadge';

interface VideoListProps {
  onVideoSelect?: (videoId: string) => void;
}

export default function VideoList({ onVideoSelect }: VideoListProps) {
  const [videos, setVideos] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);

  const fetchVideos = async (pageNum = 1) => {
    setLoading(true);
    try {
      const response = await fetch(`/api/v1/videos?page=${pageNum}&limit=10`);
      const data = await response.json();
      
      if (data.code === 200) {
        setVideos(data.data.videos || []);
        setTotal(data.data.total || 0);
      } else {
        console.error('获取视频列表失败:', data.message);
      }
    } catch (error) {
      console.error('获取视频列表失败:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchVideos(page);
  }, [page]);

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const handleRefresh = () => {
    fetchVideos(page);
  };

  const handleDelete = async (videoId: string, videoTitle: string, e: React.MouseEvent) => {
    e.stopPropagation(); // 阻止事件冒泡，避免触发视频选择
    
    if (!confirm(`确定要删除视频 "${videoTitle || videoId}" 吗？\n\n此操作将删除：\n- 所有任务步骤\n- 视频文件和字幕文件\n- 数据库记录\n\n此操作无法恢复！`)) {
      return;
    }

    try {
      const response = await fetch(`/api/v1/videos/${videoId}`, {
        method: 'DELETE',
      });

      const data = await response.json();

      if (data.code === 200) {
        alert('✅ 删除成功！');
        // 刷新列表
        fetchVideos(page);
      } else {
        alert(`❌ 删除失败：${data.message}`);
      }
    } catch (error) {
      console.error('删除视频失败:', error);
      alert('❌ 网络错误，删除失败');
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="flex flex-col items-center space-y-2">
          <RefreshCw className="w-8 h-8 animate-spin text-blue-500" />
          <span className="text-gray-500">加载中...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-lg shadow-md">
      {/* 头部 */}
      <div className="p-6 border-b border-gray-200">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-semibold text-gray-900">
            视频列表
          </h2>
          <button
            onClick={handleRefresh}
            className="flex items-center space-x-2 px-3 py-1 text-sm text-gray-600 hover:text-gray-900 transition-colors"
          >
            <RefreshCw className="w-4 h-4" />
            <span>刷新</span>
          </button>
        </div>
        
        {total > 0 && (
          <p className="text-sm text-gray-500 mt-1">
            共 {total} 个视频
          </p>
        )}
      </div>

      {/* 视频列表 */}
      <div className="divide-y divide-gray-200">
        {videos.length === 0 ? (
          <div className="p-8 text-center">
            <Play className="w-12 h-12 mx-auto text-gray-400 mb-4" />
            <p className="text-gray-500">暂无视频</p>
          </div>
        ) : (
          videos.map((video) => {
            return (
              <div
                key={video.id}
                className="p-6 hover:bg-gray-50 cursor-pointer transition-colors"
                onClick={() => onVideoSelect?.(video.video_id)}
              >
                <div className="flex items-start space-x-4">
                  {/* 视频缩略图占位 */}
                  <div className="w-20 h-12 bg-gray-200 rounded flex items-center justify-center flex-shrink-0">
                    <Play className="w-6 h-6 text-gray-400" />
                  </div>
                  
                  {/* 视频信息 */}
                  <div className="flex-1 min-w-0">
                    <h3 className="text-sm font-medium text-gray-900 truncate">
                      {video.generated_title || video.title || '未设置标题'}
                    </h3>
                    {video.generated_title && video.generated_title !== video.title && (
                      <p className="text-xs text-gray-400 truncate">
                        原标题: {video.title}
                      </p>
                    )}
                    <p className="text-xs text-gray-500 mt-1">
                      ID: {video.video_id}
                    </p>
                    <p className="text-xs text-gray-500">
                      {formatDate(video.created_at)}
                    </p>
                    {video.generated_tags && (
                      <div className="flex flex-wrap gap-1 mt-1">
                        {video.generated_tags.split(',').slice(0, 3).map((tag: string, index: number) => (
                          <span
                            key={index}
                            className="inline-block px-1 py-0.5 text-xs bg-blue-100 text-blue-800 rounded"
                          >
                            {tag.trim()}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                  
                  {/* 状态和操作按钮 */}
                  <div className="flex flex-col items-end space-y-2">
                    <StatusBadge status={video.status} />
                    
                    {video.bili_bvid && (
                      <a
                        href={`https://www.bilibili.com/video/${video.bili_bvid}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-xs text-blue-600 hover:text-blue-800 underline"
                        onClick={(e) => e.stopPropagation()}
                      >
                        访问B站
                      </a>
                    )}
                    
                    {/* 删除按钮 */}
                    <button
                      onClick={(e) => handleDelete(video.video_id, video.generated_title || video.title, e)}
                      className="flex items-center space-x-1 px-2 py-1 text-xs text-red-600 hover:text-red-800 hover:bg-red-50 rounded transition-colors"
                      title="删除视频"
                    >
                      <Trash2 className="w-3 h-3" />
                      <span>删除</span>
                    </button>
                  </div>
                </div>
                
                {/* URL信息 */}
                <div className="mt-2 ml-24">
                  <p className="text-xs text-gray-400 truncate">
                    {video.url}
                  </p>
                </div>
              </div>
            );
          })
        )}
      </div>

      {/* 分页 */}
      {total > 10 && (
        <div className="p-4 border-t border-gray-200">
          <div className="flex items-center justify-between">
            <button
              onClick={() => setPage(page - 1)}
              disabled={page === 1}
              className="px-3 py-1 text-sm border border-gray-300 rounded disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
            >
              上一页
            </button>
            
            <span className="text-sm text-gray-600">
              第 {page} 页，共 {Math.ceil(total / 10)} 页
            </span>
            
            <button
              onClick={() => setPage(page + 1)}
              disabled={page >= Math.ceil(total / 10)}
              className="px-3 py-1 text-sm border border-gray-300 rounded disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50"
            >
              下一页
            </button>
          </div>
        </div>
      )}
    </div>
  );
}