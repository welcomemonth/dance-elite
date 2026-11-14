import { message } from 'antd';
import { history } from '@umijs/max';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { clearSession, saveSession } from '@/services/youwumeng/auth';
import { errorConfig } from './requestErrorConfig';

vi.mock('antd', () => ({
  message: {
    error: vi.fn(),
  },
}));

vi.mock('@umijs/max', () => ({
  history: {
    location: {
      pathname: '/events',
      search: '?page=1',
      hash: '',
    },
    replace: vi.fn(),
  },
}));

describe('requestErrorConfig', () => {
  // biome-ignore lint/style/noNonNullAssertion: config handlers are always defined
  const errorThrower = errorConfig.errorConfig!.errorThrower!;
  // biome-ignore lint/style/noNonNullAssertion: config handlers are always defined
  const errorHandler = errorConfig.errorConfig!.errorHandler!;
  // biome-ignore lint/style/noNonNullAssertion: interceptors are always defined
  const requestInterceptor = errorConfig.requestInterceptors![0] as (config: {
    headers?: Record<string, string>;
  }) => { headers?: Record<string, string> };

  beforeEach(() => {
    vi.clearAllMocks();
    clearSession();
    history.location.pathname = '/events';
    history.location.search = '?page=1';
  });

  describe('errorThrower', () => {
    it('业务 code !== 0 时抛 BizError 并携带 info', () => {
      const res = { code: 41005, message: '名额已满', data: null };

      try {
        errorThrower(res);
        expect.unreachable('should have thrown');
      } catch (error: any) {
        expect(error.name).toBe('BizError');
        expect(error.info).toEqual({ code: 41005, message: '名额已满', data: null });
      }
    });

    it('code === 0 时不抛错', () => {
      expect(() => {
        errorThrower({ code: 0, message: 'success', data: { id: 1 } });
      }).not.toThrow();
    });
  });

  describe('errorHandler', () => {
    it('skipErrorHandler 时原样抛出', () => {
      const error = new Error('boom');
      expect(() => errorHandler(error, { skipErrorHandler: true })).toThrow('boom');
    });

    it('BizError 40101 → 清理会话并跳转登录页', () => {
      saveSession('stale-token', { id: 1 } as never);
      const error: any = new Error('未登录或登录已过期');
      error.name = 'BizError';
      error.info = { code: 40101, message: '未登录或登录已过期' };

      errorHandler(error, {});

      expect(history.replace).toHaveBeenCalledWith(
        '/user/login?redirect=%2Fevents%3Fpage%3D1',
      );
      expect(localStorage.getItem('yw_admin_token')).toBeNull();
      expect(message.error).not.toHaveBeenCalled();
    });

    it('BizError 其他业务码 → message.error 提示', () => {
      const error: any = new Error('名额已满');
      error.name = 'BizError';
      error.info = { code: 41005, message: '名额已满' };

      errorHandler(error, {});

      expect(message.error).toHaveBeenCalledWith('名额已满');
      expect(history.replace).not.toHaveBeenCalled();
    });

    it('HTTP 401 + code 40101（token 失效）→ 清理会话并跳转登录页', () => {
      saveSession('stale-token', { id: 1 } as never);
      const error: any = new Error('Request failed with status code 401');
      error.response = {
        status: 401,
        data: { code: 40101, message: '未登录或登录已过期', data: null },
      };

      errorHandler(error, {});

      expect(history.replace).toHaveBeenCalledWith(
        '/user/login?redirect=%2Fevents%3Fpage%3D1',
      );
      expect(localStorage.getItem('yw_admin_token')).toBeNull();
      expect(message.error).not.toHaveBeenCalled();
    });

    it('已跳到登录页后并发 40101 → 不再重定向（避免登录后回跳登录页）', () => {
      history.location.pathname = '/user/login';
      history.location.search = '?redirect=%2Fevents';
      const error: any = new Error('未登录或登录已过期');
      error.response = {
        status: 401,
        data: { code: 40101, message: '未登录或登录已过期', data: null },
      };

      errorHandler(error, {});

      expect(history.replace).not.toHaveBeenCalled();
      expect(message.error).not.toHaveBeenCalled();
    });

    it('HTTP 401 + 登录失败业务码（40103 密码错误）→ 提示信息，不跳转', () => {
      const error: any = new Error('Request failed with status code 401');
      error.response = {
        status: 401,
        data: { code: 40103, message: '密码错误', data: null },
      };

      errorHandler(error, {});

      expect(message.error).toHaveBeenCalledWith('密码错误');
      expect(history.replace).not.toHaveBeenCalled();
    });

    it('HTTP 403 + 40302（账号已停用）→ 提示后端信息，不跳转', () => {
      const error: any = new Error('Request failed with status code 403');
      error.response = {
        status: 403,
        data: { code: 40302, message: '账号已停用', data: null },
      };

      errorHandler(error, {});

      expect(message.error).toHaveBeenCalledWith('账号已停用');
      expect(history.replace).not.toHaveBeenCalled();
    });

    it('其他 HTTP 错误 → 提示状态码', () => {
      const error: any = new Error('Request failed with status code 500');
      error.response = { status: 500, data: null };

      errorHandler(error, {});

      expect(message.error).toHaveBeenCalledWith('请求失败：HTTP 500');
    });

    it('无 response（网络异常）→ 通用提示', () => {
      errorHandler(new Error('Network Error'), {});

      expect(message.error).toHaveBeenCalledWith('网络异常，请重试');
    });
  });

  describe('requestInterceptors', () => {
    it('有 token 时注入 Authorization 头', () => {
      saveSession('abc-token', { id: 1 } as never);

      const result = requestInterceptor({});

      expect(result.headers?.Authorization).toBe('Bearer abc-token');
    });

    it('无 token 时不注入 Authorization 头', () => {
      const result = requestInterceptor({});

      expect(result.headers?.Authorization).toBeUndefined();
    });
  });
});
