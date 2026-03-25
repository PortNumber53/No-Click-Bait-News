import { useState, useEffect } from 'react';
import { api } from '../services/api';
import type { ComparisonData, VoteStats } from '../types';
import './ComparisonCard.css';

interface Props {
  comparison: ComparisonData;
  showContent?: boolean;
}

export function ComparisonCard({ comparison, showContent = false }: Props) {
  const [vote, setVote] = useState<'a' | 'b' | null>(comparison.user_vote);
  const [stats, setStats] = useState<VoteStats | null>(null);
  const [voting, setVoting] = useState(false);

  const handleVote = async (choice: 'a' | 'b') => {
    if (vote || voting) return;
    setVoting(true);
    try {
      const chosen = choice === 'a' ? comparison.version_a : comparison.version_b;
      const other = choice === 'a' ? comparison.version_b : comparison.version_a;
      const result = await api.submitVote(comparison.article_id, chosen.id, other.id);
      setVote(choice);
      setStats(result);
    } catch {
      // silently fail
    } finally {
      setVoting(false);
    }
  };

  useEffect(() => {
    if (vote && !stats) {
      api.getVoteStats(comparison.article_id).then(setStats).catch(() => {});
    }
  }, [vote, stats, comparison.article_id]);

  const totalVotes = stats ? stats.version_a_votes + stats.version_b_votes : 0;

  const getModelName = (versionId: string) => {
    if (!stats) return '';
    if (versionId === stats.version_a_id) return stats.version_a_name;
    if (versionId === stats.version_b_id) return stats.version_b_name;
    return '';
  };

  const getVoteCount = (versionId: string) => {
    if (!stats) return 0;
    if (versionId === stats.version_a_id) return stats.version_a_votes;
    if (versionId === stats.version_b_id) return stats.version_b_votes;
    return 0;
  };

  const getPercent = (versionId: string) => {
    if (!totalVotes) return 0;
    return Math.round((getVoteCount(versionId) / totalVotes) * 100);
  };

  const renderVersion = (key: 'a' | 'b') => {
    const version = key === 'a' ? comparison.version_a : comparison.version_b;
    const isChosen = vote === key;
    return (
      <div className={`comparison__version ${isChosen ? 'comparison__version--chosen' : ''} ${vote ? 'comparison__version--voted' : ''}`}>
        <span className="comparison__version-label">Version {key.toUpperCase()}</span>
        <h4 className="comparison__version-title">{version.title}</h4>
        <p className="comparison__version-summary">{version.summary}</p>
        {showContent && version.content && (
          <p className="comparison__version-content">{version.content}</p>
        )}

        <div className="comparison__version-footer">
          {!vote ? (
            <button
              className="btn btn--filled comparison__vote-btn"
              onClick={() => handleVote(key)}
              disabled={voting}
            >
              {voting ? 'Voting...' : 'I prefer this'}
            </button>
          ) : (
            <div className="comparison__result">
              {stats && (
                <>
                  <div className="comparison__bar-bg">
                    <div
                      className="comparison__bar-fill"
                      style={{ width: `${getPercent(version.id)}%` }}
                    />
                  </div>
                  <span className="comparison__stat">
                    {getPercent(version.id)}% ({getVoteCount(version.id)} votes)
                  </span>
                  <span className="comparison__model">{getModelName(version.id)}</span>
                </>
              )}
              {isChosen && <span className="comparison__your-pick">Your pick</span>}
            </div>
          )}
        </div>
      </div>
    );
  };

  return (
    <div className="comparison">
      <h3 className="comparison__label">Which rewrite is better?</h3>

      <div className="comparison__grid">
        {renderVersion('a')}

        <div className="comparison__original">
          <span className="comparison__version-label">Original</span>
          <h4 className="comparison__version-title">{comparison.original_title}</h4>
          <p className="comparison__version-summary">
            {comparison.source_name}
          </p>
        </div>

        {renderVersion('b')}
      </div>
    </div>
  );
}
