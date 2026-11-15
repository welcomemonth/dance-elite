interface IAppOption {
  globalData: {
    /** 本次冷启动时间戳，便于排查问题。 */
    launchTime: number;
  };
}
