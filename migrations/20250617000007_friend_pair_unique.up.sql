DELETE FROM friends
WHERE (user_id, friend_id) IN (
    SELECT user_id, friend_id
    FROM (
        SELECT user_id,
               friend_id,
               ROW_NUMBER() OVER (
                   PARTITION BY LEAST(user_id, friend_id), GREATEST(user_id, friend_id)
                   ORDER BY CASE status
                                WHEN 'accepted' THEN 0
                                WHEN 'pending' THEN 1
                                ELSE 2
                            END,
                            request_at ASC,
                            user_id ASC
               ) AS rn
        FROM friends
    ) ranked
    WHERE rn > 1
);

CREATE UNIQUE INDEX unique_friend_pair
    ON friends (LEAST(user_id, friend_id), GREATEST(user_id, friend_id));
