export type EnvVersion = "develop" | "trial" | "release";

interface EnvConfig {
  apiBaseUrl: string;
  requestTimeout: number;
}

const ENV_CONFIG: Record<EnvVersion, EnvConfig> = {
  // 本地联调：微信开发者工具需勾选「不校验合法域名」。
  develop: {
    apiBaseUrl: "https://dance-test.meetmonth.top",
    requestTimeout: 15000,
  },
  trial: {
    apiBaseUrl: "https://dance-test.meetmonth.top",
    requestTimeout: 15000,
  },
  release: {
    apiBaseUrl: "https://dance-test.meetmonth.top",
    requestTimeout: 15000,
  },
};

function detectEnvVersion(): EnvVersion {
  try {
    return wx.getAccountInfoSync().miniProgram.envVersion;
  } catch {
    return "develop";
  }
}

export const ENV_VERSION: EnvVersion = detectEnvVersion();

export const API_BASE_URL: string = ENV_CONFIG[ENV_VERSION].apiBaseUrl;

export const REQUEST_TIMEOUT: number = ENV_CONFIG[ENV_VERSION].requestTimeout;

/** 是否输出请求链路日志（仅开发环境）。 */
export const DEBUG: boolean = ENV_VERSION === "develop";
