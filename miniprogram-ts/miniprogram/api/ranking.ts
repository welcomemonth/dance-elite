import type {
  DancerDetail,
  InstitutionRanking,
  RankingBoard,
  RankingConfig,
} from "../types/models";
import { http } from "../utils/request";

/** 榜单首页配置（GET /api/v1/config，API 文档 6.1，免鉴权）。 */
export function getRankingConfig(): Promise<RankingConfig> {
  return http.get<RankingConfig>("/api/v1/config", undefined, { auth: false });
}

/** 指定榜单的直通榜与积分榜（GET /api/v1/ranking，API 文档 6.2）。 */
export function getRanking(level: string, danceType: string): Promise<RankingBoard> {
  return http.get<RankingBoard>(
    "/api/v1/ranking",
    { level, dance_type: danceType },
    { auth: false },
  );
}

/** 机构年度榜（GET /api/v1/institution/ranking，API 文档 6.3）。 */
export function getInstitutionRanking(): Promise<InstitutionRanking> {
  return http.get<InstitutionRanking>("/api/v1/institution/ranking", undefined, {
    auth: false,
  });
}

/** 选手积分详情（GET /api/v1/dancers/{entry_id}，API 文档 6.4）。 */
export function getDancerDetail(entryId: number): Promise<DancerDetail> {
  return http.get<DancerDetail>(`/api/v1/dancers/${entryId}`, undefined, {
    auth: false,
  });
}
